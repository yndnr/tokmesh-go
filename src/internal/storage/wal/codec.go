package wal

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

type wirePayload struct {
	Timestamp int64  `json:"ts"`
	SessionID string `json:"sid"`
	Version   uint64 `json:"ver,omitempty"`

	Session *domain.Session `json:"session,omitempty"`

	// EncryptedSession is base64 of adaptive.Cipher.Encrypt(sessionJSON).
	EncryptedSession string `json:"enc_session,omitempty"`
}

func encodeEntryFrame(e *Entry, cipher adaptive.Cipher) ([]byte, error) {
	if e == nil {
		return nil, fmt.Errorf("wal: entry is nil")
	}
	if e.OpType == OpTypeUnspecified {
		return nil, ErrInvalidEntryType
	}
	if e.OpType != OpTypeDelete && e.Session == nil {
		return nil, fmt.Errorf("wal: missing session for op %d", e.OpType)
	}

	p := wirePayload{
		Timestamp: e.Timestamp,
		SessionID: e.SessionID,
		Version:   e.Version,
	}

	if e.OpType != OpTypeDelete {
		if cipher == nil {
			p.Session = e.Session
		} else {
			plainSession, err := json.Marshal(e.Session)
			if err != nil {
				return nil, fmt.Errorf("wal: marshal session: %w", err)
			}
			encrypted, err := cipher.Encrypt(plainSession, nil)
			if err != nil {
				return nil, fmt.Errorf("wal: encrypt session: %w", err)
			}
			p.EncryptedSession = base64.StdEncoding.EncodeToString(encrypted)
		}
	}

	payload, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("wal: marshal payload: %w", err)
	}

	typeByte := []byte{byte(e.OpType)}
	crc := crc32.ChecksumIEEE(append(typeByte, payload...))

	// Length = CRC(4) + Type(1) + Payload.
	length := uint32(4 + 1 + len(payload))
	if length < 5 {
		return nil, ErrCorruptedEntry
	}

	out := make([]byte, 0, 4+int(length))
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], length)
	out = append(out, header[:]...)

	var crcBuf [4]byte
	binary.BigEndian.PutUint32(crcBuf[:], crc)
	out = append(out, crcBuf[:]...)

	out = append(out, typeByte...)
	out = append(out, payload...)
	return out, nil
}

func decodeEntryFrame(frame []byte, cipher adaptive.Cipher) (*Entry, error) {
	// Frame layout: [crc32:4][type:1][payload...]
	if len(frame) < 5 {
		return nil, ErrCorruptedEntry
	}

	wantCRC := binary.BigEndian.Uint32(frame[:4])
	typeByte := frame[4]
	payload := frame[5:]

	gotCRC := crc32.ChecksumIEEE(append([]byte{typeByte}, payload...))
	if gotCRC != wantCRC {
		return nil, ErrChecksumMismatch
	}

	var p wirePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("wal: unmarshal payload: %w", err)
	}

	op := OpType(typeByte)
	switch op {
	case OpTypeCreate, OpTypeUpdate, OpTypeDelete:
	default:
		return nil, ErrInvalidEntryType
	}

	out := &Entry{
		OpType:    op,
		Timestamp: p.Timestamp,
		SessionID: p.SessionID,
		Version:   p.Version,
	}

	if op == OpTypeDelete {
		return out, nil
	}

	if p.Session != nil {
		out.Session = p.Session
		return out, nil
	}

	if p.EncryptedSession == "" {
		return nil, fmt.Errorf("wal: missing session payload")
	}
	if cipher == nil {
		return nil, fmt.Errorf("wal: encrypted entry requires cipher")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(p.EncryptedSession)
	if err != nil {
		return nil, fmt.Errorf("wal: decode encrypted session: %w", err)
	}

	plain, err := cipher.Decrypt(ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("wal: decrypt session: %w", err)
	}

	var sess domain.Session
	if err := json.Unmarshal(plain, &sess); err != nil {
		return nil, fmt.Errorf("wal: unmarshal session: %w", err)
	}
	out.Session = &sess
	return out, nil
}

