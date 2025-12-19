// Package snapshot provides snapshot management for TokMesh.
//
// @req RQ-0101
// @design DS-0102
package snapshot

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

// Magic bytes identify snapshot files (DS-0102).
var magicBytes = []byte("TOKMSNAP")

const (
	filePrefix    = "snapshot-"
	fileExtension = ".snap"
	checksumSize  = 32
	headerVersion = 1

	DefaultRetentionCount = 5
	DefaultRetentionDays  = 7
)

type snapshotHeader struct {
	Version       int    `json:"version"`
	CreatedAt     int64  `json:"created_at"`
	NodeID        string `json:"node_id,omitempty"`
	SessionCount  uint64 `json:"session_count"`
	WALLastOffset uint64 `json:"wal_last_offset"`
	Encrypted     bool   `json:"encrypted"`
}

type snapshotSession struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	TokenHash    string            `json:"token_hash"`
	IPAddress    string            `json:"ip_address"`
	UserAgent    string            `json:"user_agent"`
	LastAccessIP string            `json:"last_access_ip"`
	LastAccessUA string            `json:"last_access_ua"`
	DeviceID     string            `json:"device_id"`
	CreatedBy    string            `json:"created_by"`
	CreatedAt    int64             `json:"created_at"`
	ExpiresAt    int64             `json:"expires_at"`
	LastActive   int64             `json:"last_active"`
	Data         map[string]string `json:"data"`
	Version      uint64            `json:"version"`

	ShardID   uint32 `json:"shard_id"`
	TTL       int64  `json:"ttl"`
	IsDeleted bool   `json:"is_deleted"`
}

func snapshotSessionFromDomain(s *domain.Session) snapshotSession {
	return snapshotSession{
		ID:           s.ID,
		UserID:       s.UserID,
		TokenHash:    s.TokenHash,
		IPAddress:    s.IPAddress,
		UserAgent:    s.UserAgent,
		LastAccessIP: s.LastAccessIP,
		LastAccessUA: s.LastAccessUA,
		DeviceID:     s.DeviceID,
		CreatedBy:    s.CreatedBy,
		CreatedAt:    s.CreatedAt,
		ExpiresAt:    s.ExpiresAt,
		LastActive:   s.LastActive,
		Data:         s.Data,
		Version:      s.Version,
		ShardID:      s.ShardID,
		TTL:          s.TTL,
		IsDeleted:    s.IsDeleted,
	}
}

func (s snapshotSession) toDomain() *domain.Session {
	return &domain.Session{
		ID:           s.ID,
		UserID:       s.UserID,
		TokenHash:    s.TokenHash,
		IPAddress:    s.IPAddress,
		UserAgent:    s.UserAgent,
		LastAccessIP: s.LastAccessIP,
		LastAccessUA: s.LastAccessUA,
		DeviceID:     s.DeviceID,
		CreatedBy:    s.CreatedBy,
		CreatedAt:    s.CreatedAt,
		ExpiresAt:    s.ExpiresAt,
		LastActive:   s.LastActive,
		Data:         s.Data,
		Version:      s.Version,
		ShardID:      s.ShardID,
		TTL:          s.TTL,
		IsDeleted:    s.IsDeleted,
	}
}

var (
	ErrInvalidMagic     = errors.New("snapshot: invalid magic bytes")
	ErrChecksumMismatch = errors.New("snapshot: checksum mismatch")
	ErrNotFound         = errors.New("snapshot: not found")
	ErrNoSnapshots      = errors.New("snapshot: no snapshots available")
)

// Config configures the snapshot manager.
type Config struct {
	Dir string

	RetentionCount int
	RetentionDays  int

	Cipher adaptive.Cipher
	NodeID string
}

func DefaultConfig(dir string) Config {
	return Config{
		Dir:            dir,
		RetentionCount: DefaultRetentionCount,
		RetentionDays:  DefaultRetentionDays,
	}
}

type Manager struct {
	cfg    Config
	cipher adaptive.Cipher
}

func NewManager(cfg Config) (*Manager, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("snapshot: dir is required")
	}
	if err := os.MkdirAll(cfg.Dir, 0750); err != nil {
		return nil, fmt.Errorf("snapshot: create dir: %w", err)
	}
	if cfg.RetentionCount == 0 {
		cfg.RetentionCount = DefaultRetentionCount
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = DefaultRetentionDays
	}

	return &Manager{
		cfg:    cfg,
		cipher: cfg.Cipher,
	}, nil
}

// Info contains metadata about a snapshot.
type Info struct {
	ID string `json:"id"`

	// WALLastOffset is the WAL composite offset covered by this snapshot.
	// Format: (segmentID<<32 | offsetWithinSegment).
	WALLastOffset uint64 `json:"wal_last_offset"`

	SessionCount int64  `json:"session_count"`
	CreatedAt    int64  `json:"created_at"`
	Size         int64  `json:"size"`
	Path         string `json:"path"`
	Checksum     string `json:"checksum"`
	NodeID       string `json:"node_id,omitempty"`
}

// Create creates a new snapshot file from the given sessions.
func (m *Manager) Create(sessions []*domain.Session, walLastOffset uint64) (*Info, error) {
	now := time.Now()
	id := m.generateID(now)

	tempPath := filepath.Join(m.cfg.Dir, id+".tmp")
	file, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("snapshot: create temp file: %w", err)
	}
	defer os.Remove(tempPath)

	hash := sha256.New()
	writer := io.MultiWriter(file, hash)

	if _, err := writer.Write(magicBytes); err != nil {
		file.Close()
		return nil, err
	}

	hdr := snapshotHeader{
		Version:       headerVersion,
		CreatedAt:     now.UnixMilli(),
		NodeID:        m.cfg.NodeID,
		SessionCount:  uint64(len(sessions)),
		WALLastOffset: walLastOffset,
		Encrypted:     m.cipher != nil,
	}

	hdrJSON, err := json.Marshal(hdr)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: marshal header: %w", err)
	}

	var hdrLen [4]byte
	binary.BigEndian.PutUint32(hdrLen[:], uint32(len(hdrJSON)))
	if _, err := writer.Write(hdrLen[:]); err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: write header length: %w", err)
	}
	if _, err := writer.Write(hdrJSON); err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: write header: %w", err)
	}

	encoded := make([]snapshotSession, 0, len(sessions))
	for _, s := range sessions {
		encoded = append(encoded, snapshotSessionFromDomain(s))
	}

	data, err := json.Marshal(encoded)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: marshal sessions: %w", err)
	}
	if m.cipher != nil {
		data, err = m.cipher.Encrypt(data, nil)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("snapshot: encrypt: %w", err)
		}
	}

	var dataLen [4]byte
	binary.BigEndian.PutUint32(dataLen[:], uint32(len(data)))
	if _, err := writer.Write(dataLen[:]); err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: write data length: %w", err)
	}
	if _, err := writer.Write(data); err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: write data: %w", err)
	}

	// Finalize checksum trailer (not included in hash).
	sum := hash.Sum(nil)
	if len(sum) != checksumSize {
		file.Close()
		return nil, fmt.Errorf("snapshot: invalid sha256 size: %d", len(sum))
	}
	if _, err := file.Write(sum); err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: write checksum: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("snapshot: sync: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("snapshot: close: %w", err)
	}

	stat, err := os.Stat(tempPath)
	if err != nil {
		return nil, err
	}

	finalPath := filepath.Join(m.cfg.Dir, id+fileExtension)
	if err := os.Rename(tempPath, finalPath); err != nil {
		return nil, fmt.Errorf("snapshot: rename: %w", err)
	}

	return &Info{
		ID:            id,
		WALLastOffset: walLastOffset,
		SessionCount:  int64(len(sessions)),
		CreatedAt:     now.UnixMilli(),
		Size:          stat.Size(),
		Path:          finalPath,
		Checksum:      hex.EncodeToString(sum),
		NodeID:        m.cfg.NodeID,
	}, nil
}

// Load loads sessions from the latest valid snapshot.
// If the latest snapshot is corrupted, it falls back to older snapshots.
func (m *Manager) Load() ([]*domain.Session, *Info, error) {
	snapshots, err := m.List()
	if err != nil {
		return nil, nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil, ErrNoSnapshots
	}

	for i := len(snapshots) - 1; i >= 0; i-- {
		sessions, info, err := m.loadFile(snapshots[i].Path)
		if err == nil {
			return sessions, info, nil
		}
		if errors.Is(err, ErrChecksumMismatch) || errors.Is(err, ErrInvalidMagic) {
			continue
		}
		return nil, nil, err
	}

	return nil, nil, ErrNoSnapshots
}

func (m *Manager) loadFile(path string) ([]*domain.Session, *Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if stat.Size() < int64(len(magicBytes))+checksumSize {
		return nil, nil, ErrChecksumMismatch
	}

	// Verify checksum.
	dataLen := stat.Size() - checksumSize
	expected := make([]byte, checksumSize)
	if _, err := io.ReadFull(io.NewSectionReader(f, dataLen, checksumSize), expected); err != nil {
		return nil, nil, err
	}
	h := sha256.New()
	if _, err := io.CopyN(h, io.NewSectionReader(f, 0, dataLen), dataLen); err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(h.Sum(nil), expected) {
		return nil, nil, ErrChecksumMismatch
	}

	br := bufio.NewReader(io.NewSectionReader(f, 0, dataLen))

	magic := make([]byte, len(magicBytes))
	if _, err := io.ReadFull(br, magic); err != nil {
		return nil, nil, err
	}
	if !bytes.Equal(magic, magicBytes) {
		return nil, nil, ErrInvalidMagic
	}

	var hdrLenBuf [4]byte
	if _, err := io.ReadFull(br, hdrLenBuf[:]); err != nil {
		return nil, nil, err
	}
	hdrLen := binary.BigEndian.Uint32(hdrLenBuf[:])
	if hdrLen == 0 {
		return nil, nil, fmt.Errorf("snapshot: empty header")
	}
	hdrJSON := make([]byte, hdrLen)
	if _, err := io.ReadFull(br, hdrJSON); err != nil {
		return nil, nil, err
	}

	var hdr snapshotHeader
	if err := json.Unmarshal(hdrJSON, &hdr); err != nil {
		return nil, nil, fmt.Errorf("snapshot: unmarshal header: %w", err)
	}

	var dataLenBuf [4]byte
	if _, err := io.ReadFull(br, dataLenBuf[:]); err != nil {
		return nil, nil, err
	}
		dataSize := binary.BigEndian.Uint32(dataLenBuf[:])
		data := make([]byte, dataSize)
	if _, err := io.ReadFull(br, data); err != nil {
		return nil, nil, err
	}

		var sessions []*domain.Session
		if hdr.Encrypted {
			if m.cipher == nil {
				// Compatibility behavior: allow loading metadata without decrypting data.
				// This matches the previous "encrypted block with nil Sessions" semantics.
				sessions = nil
			} else {
				plain, err := m.cipher.Decrypt(data, nil)
				if err != nil {
					return nil, nil, fmt.Errorf("snapshot: decrypt: %w", err)
				}
				data = plain
			}
		} else if m.cipher != nil {
			return nil, nil, fmt.Errorf("snapshot: expected encrypted snapshot")
		}

		if sessions == nil && hdr.Encrypted && m.cipher == nil {
			// Skip decoding the encrypted data block.
		} else {
			var decoded []snapshotSession
			if err := json.Unmarshal(data, &decoded); err != nil {
				return nil, nil, fmt.Errorf("snapshot: unmarshal sessions: %w", err)
			}
			sessions = make([]*domain.Session, 0, len(decoded))
			for _, s := range decoded {
				sessions = append(sessions, s.toDomain())
			}
		}

	info := &Info{
		ID:            strings.TrimSuffix(filepath.Base(path), fileExtension),
		WALLastOffset: hdr.WALLastOffset,
		SessionCount:  int64(hdr.SessionCount),
		CreatedAt:     hdr.CreatedAt,
		Size:          stat.Size(),
		Path:          path,
		Checksum:      hex.EncodeToString(expected),
		NodeID:        hdr.NodeID,
	}

	return sessions, info, nil
}

// List lists snapshot files (metadata only).
func (m *Manager) List() ([]*Info, error) {
	entries, err := os.ReadDir(m.cfg.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, filePrefix) && strings.HasSuffix(name, fileExtension) {
			paths = append(paths, filepath.Join(m.cfg.Dir, name))
		}
	}
	sort.Strings(paths)

	var infos []*Info
	for _, p := range paths {
		stat, err := os.Stat(p)
		if err != nil {
			continue
		}
		infos = append(infos, &Info{
			ID:   strings.TrimSuffix(filepath.Base(p), fileExtension),
			Path: p,
			Size: stat.Size(),
		})
	}
	return infos, nil
}

// Prune applies the retention policy and deletes old snapshots.
func (m *Manager) Prune() error {
	infos, err := m.List()
	if err != nil {
		return err
	}
	if len(infos) <= 1 {
		return nil
	}

	keep := make(map[string]struct{}, len(infos))

	// Keep last RetentionCount.
	if m.cfg.RetentionCount > 0 {
		start := len(infos) - m.cfg.RetentionCount
		if start < 0 {
			start = 0
		}
		for _, info := range infos[start:] {
			keep[info.Path] = struct{}{}
		}
	}

	// Keep those within RetentionDays based on mtime.
	if m.cfg.RetentionDays > 0 {
		cutoff := time.Now().Add(-time.Duration(m.cfg.RetentionDays) * 24 * time.Hour)
		for _, info := range infos {
			st, err := os.Stat(info.Path)
			if err != nil {
				continue
			}
			if st.ModTime().After(cutoff) {
				keep[info.Path] = struct{}{}
			}
		}
	}

	// Always keep at least the newest.
	keep[infos[len(infos)-1].Path] = struct{}{}

	for _, info := range infos {
		if _, ok := keep[info.Path]; ok {
			continue
		}
		_ = os.Remove(info.Path)
	}
	return nil
}

func (m *Manager) generateID(t time.Time) string {
	ts := t.Format("20060102150405")
	seq := 1

	entries, _ := os.ReadDir(m.cfg.Dir)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filePrefix+ts+"-") || !strings.HasSuffix(name, fileExtension) {
			continue
		}
		seq++
	}

	return fmt.Sprintf("%s%s-%04d", filePrefix, ts, seq)
}
