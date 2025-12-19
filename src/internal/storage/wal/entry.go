// Package wal provides Write-Ahead Logging for durability.
package wal

import (
	"errors"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
)

// File format constants.
const (
	// DefaultFileExtension is the WAL file extension.
	DefaultFileExtension = ".wal"

	// headerSize is the size of entry header: length (4) + crc (4) = 8 bytes.
	headerSize = 8

	// minEntrySize is the minimum entry size: header (8) + type (1).
	minEntrySize = headerSize + 1
)

// Errors for WAL operations.
var (
	ErrCorruptedEntry   = errors.New("wal: corrupted entry")
	ErrChecksumMismatch = errors.New("wal: checksum mismatch")
	ErrInvalidEntryType = errors.New("wal: invalid entry type")
)

// OpType represents the type of operation in the WAL.
type OpType uint8

const (
	OpTypeUnspecified OpType = iota
	OpTypeCreate
	OpTypeUpdate
	OpTypeDelete
)

// Legacy type alias for backward compatibility.
type EntryType = OpType

// Legacy constants for backward compatibility.
const (
	EntryTypeCreate = OpTypeCreate
	EntryTypeUpdate = OpTypeUpdate
	EntryTypeDelete = OpTypeDelete
)

// Entry represents one durable operation written to the WAL.
//
// Timestamp uses Unix milliseconds to match the Protobuf schema.
type Entry struct {
	OpType    OpType
	Timestamp int64
	SessionID string
	Version   uint64
	Session   *domain.Session
}

// NewCreateEntry creates a CREATE WAL entry.
func NewCreateEntry(session *domain.Session) *Entry {
	return &Entry{
		OpType:    OpTypeCreate,
		Timestamp: time.Now().UnixMilli(),
		SessionID: session.ID,
		Version:   session.Version,
		Session:   session,
	}
}

// NewUpdateEntry creates an UPDATE WAL entry.
func NewUpdateEntry(session *domain.Session) *Entry {
	return &Entry{
		OpType:    OpTypeUpdate,
		Timestamp: time.Now().UnixMilli(),
		SessionID: session.ID,
		Version:   session.Version,
		Session:   session,
	}
}

// NewDeleteEntry creates a DELETE WAL entry.
func NewDeleteEntry(sessionID string) *Entry {
	return &Entry{
		OpType:    OpTypeDelete,
		Timestamp: time.Now().UnixMilli(),
		SessionID: sessionID,
	}
}
