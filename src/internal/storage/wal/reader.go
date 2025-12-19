// Package wal provides Write-Ahead Logging for durability.
package wal

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

var (
	ErrCorrupted = errors.New("wal: corrupted segment")
)

type segmentInfo struct {
	id   uint64
	path string
}

// Reader reads WAL entries across all segments in order.
type Reader struct {
	dir    string
	cipher adaptive.Cipher

	segments []segmentInfo
	segIndex int

	file     *os.File
	dataLen  int64
	startAt  int64
	reader   *bufio.Reader
	headerOK bool
}

// NewReader creates a new WAL reader for a directory.
func NewReader(dir string, cipher adaptive.Cipher) (*Reader, error) {
	r := &Reader{
		dir:    dir,
		cipher: cipher,
	}
	if err := r.scanSegments(); err != nil {
		return nil, err
	}
	return r, nil
}

// Seek positions the reader at the given composite offset.
// Offset is (segmentID<<32 | offsetWithinSegment).
func (r *Reader) Seek(offset uint64) error {
	segID := offset >> 32
	segOff := int64(uint32(offset))

	// Find segment index (first with id >= segID).
	i := 0
	for ; i < len(r.segments); i++ {
		if r.segments[i].id >= segID {
			break
		}
	}
	r.closeCurrent()
	r.segIndex = i
	r.startAt = segOff
	r.headerOK = false
	return nil
}

// Read reads the next entry from the WAL stream.
func (r *Reader) Read() (*Entry, error) {
	for {
		// Need next segment.
		if r.reader == nil {
			if err := r.openNextSegment(); err != nil {
				return nil, err
			}
		}

		// Ensure header is consumed when starting at offset 0.
		if !r.headerOK && r.startAt == 0 {
			if err := r.readAndValidateHeader(); err != nil {
				if errors.Is(err, ErrCorrupted) || errors.Is(err, errChecksumInvalid) {
					r.closeCurrent()
					continue
				}
				return nil, err
			}
		} else {
			r.headerOK = true
		}

		e, err := r.readOneEntry()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				r.closeCurrent()
				continue
			}
			if errors.Is(err, ErrCorruptedEntry) || errors.Is(err, ErrChecksumMismatch) || errors.Is(err, ErrInvalidEntryType) {
				r.closeCurrent()
				continue
			}
			return nil, err
		}
		return e, nil
	}
}

// ReadAll reads all entries from the WAL.
func (r *Reader) ReadAll() ([]*Entry, error) {
	var out []*Entry
	for {
		e, err := r.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return nil, err
		}
		out = append(out, e)
	}
}

// Close closes any open segment file.
func (r *Reader) Close() error {
	return r.closeCurrent()
}

func (r *Reader) scanSegments() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			r.segments = nil
			return nil
		}
		return err
	}

	var segs []segmentInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		id, ok := parseSegmentFilename(e.Name())
		if !ok {
			continue
		}
		segs = append(segs, segmentInfo{
			id:   id,
			path: filepath.Join(r.dir, e.Name()),
		})
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].id < segs[j].id })
	r.segments = segs
	return nil
}

func (r *Reader) openNextSegment() error {
	r.closeCurrent()

	if r.segIndex >= len(r.segments) {
		return io.EOF
	}

	seg := r.segments[r.segIndex]
	r.segIndex++

	f, err := os.Open(seg.path)
	if err != nil {
		return err
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}

	closed, dataLen, err := verifyChecksumTrailer(f, stat.Size())
	if err != nil {
		f.Close()
		return err
	}
	if closed && dataLen < MagicBytesSize {
		f.Close()
		return ErrCorrupted
	}
	r.dataLen = dataLen
	r.file = f

	// Limit reads to data portion (excluding checksum trailer if present).
	limit := r.dataLen
	if !closed {
		limit = stat.Size()
	}
	r.dataLen = limit

	// Validate checksum if present.
	if closed {
		// already verified by verifyChecksumTrailer
	}

	// Start from requested offset. If startAt==0, we will read magic+header.
	sr := io.NewSectionReader(f, r.startAt, r.dataLen-r.startAt)
	r.reader = bufio.NewReader(sr)

	r.headerOK = false

	// After first segment, subsequent segments start at 0.
	r.startAt = 0
	return nil
}

func (r *Reader) readAndValidateHeader() error {
	magic := make([]byte, MagicBytesSize)
	if _, err := io.ReadFull(r.reader, magic); err != nil {
		return err
	}
	if string(magic) != MagicBytes {
		return errInvalidMagic
	}
	r.headerOK = true
	return nil
}

func (r *Reader) closeCurrent() error {
	r.reader = nil
	r.headerOK = false

	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}

func (r *Reader) readOneEntry() (*Entry, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r.reader, lenBuf[:]); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf[:])
	if length < 5 {
		return nil, ErrCorruptedEntry
	}

	frame := make([]byte, length)
	if _, err := io.ReadFull(r.reader, frame); err != nil {
		return nil, err
	}

	return decodeEntryFrame(frame, r.cipher)
}

// VerifyTrailerChecksum is a helper used by tests.
func VerifyTrailerChecksum(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() < ChecksumSize {
		return ErrCorrupted
	}

	trailer := make([]byte, ChecksumSize)
	if _, err := io.ReadFull(io.NewSectionReader(f, stat.Size()-ChecksumSize, ChecksumSize), trailer); err != nil {
		return err
	}

	h := sha256.New()
	if _, err := io.CopyN(h, io.NewSectionReader(f, 0, stat.Size()-ChecksumSize), stat.Size()-ChecksumSize); err != nil {
		return err
	}
	if !bytes.Equal(h.Sum(nil), trailer) {
		return errChecksumInvalid
	}
	return nil
}
