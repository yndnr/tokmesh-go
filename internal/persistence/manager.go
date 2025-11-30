// Package persistence 实现 TokMesh 的 WAL、快照与持久化管理。
package persistence

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yndnr/tokmesh-go/internal/session"
)

// Manager 负责加载/记录 Session WAL 与快照。
type Manager struct {
	dataDir      string
	snapshotPath string
	wal          *wal
	cipher       dataCipher
}

// ManagerOption 自定义持久化行为。
type ManagerOption func(*Manager) error

// WithEncryptionKey 启用持久化加密。
func WithEncryptionKey(key []byte) ManagerOption {
	return func(m *Manager) error {
		if len(key) == 0 {
			return nil
		}
		c, err := newAESCipher(key)
		if err != nil {
			return err
		}
		m.cipher = c
		return nil
	}
}

// NewManager 创建会话持久化管理器并打开 WAL。
// Parameters: dataDir 为 WAL 与快照存储目录。
func NewManager(dataDir string, opts ...ManagerOption) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	mgr := &Manager{
		dataDir:      dataDir,
		snapshotPath: filepath.Join(dataDir, "sessions.snapshot"),
	}
	for _, opt := range opts {
		if err := opt(mgr); err != nil {
			return nil, err
		}
	}
	walPath := filepath.Join(dataDir, "sessions.wal")
	wal, err := openWAL(walPath, mgr.cipher)
	if err != nil {
		return nil, err
	}
	mgr.wal = wal
	return mgr, nil
}

// Load 先应用快照再回放 WAL。
func (m *Manager) Load(store *session.Store) error {
	if err := m.loadSnapshot(store); err != nil {
		return err
	}
	return m.wal.Replay(store)
}

func (m *Manager) loadSnapshot(store *session.Store) error {
	f, err := os.Open(m.snapshotPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	return readSnapshot(f, m.cipher, store)
}

// AppendUpsert 将 Session 变更事件写入 WAL，用于后续回放。
func (m *Manager) AppendUpsert(sess *session.Session) error {
	return m.wal.AppendUpsert(sess)
}

// AppendDelete 记录删除事件，保证 WAL 可恢复准确的内存视图。
func (m *Manager) AppendDelete(id string) error {
	return m.wal.AppendDelete(id)
}

// TakeSnapshot 写入全量会话，并在成功后截断 WAL。
func (m *Manager) TakeSnapshot(store *session.Store) error {
	tmpPath := m.snapshotPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := writeSnapshot(f, store.AllSessions(), m.cipher); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, m.snapshotPath); err != nil {
		return err
	}
	return m.wal.Truncate()
}

// Close 关闭 WAL 句柄并刷新缓冲区。
func (m *Manager) Close() error {
	return m.wal.Close()
}

func writeSnapshot(w io.Writer, sessions []*session.Session, cipher dataCipher) error {
	enc := json.NewEncoder(w)
	for _, sess := range sessions {
		if cipher == nil {
			if err := enc.Encode(sess); err != nil {
				return err
			}
			continue
		}
		payload, err := json.Marshal(sess)
		if err != nil {
			return err
		}
		ciphertext, err := cipher.Encrypt(payload)
		if err != nil {
			return err
		}
		record := encryptedEnvelope{Enc: base64.StdEncoding.EncodeToString(ciphertext)}
		if err := enc.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func readSnapshot(r io.Reader, cipher dataCipher, store *session.Store) error {
	dec := json.NewDecoder(r)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		sess, err := decodeSnapshotRaw(raw, cipher)
		if err != nil {
			return err
		}
		copy := *sess
		store.PutSession(&copy)
	}
}

func decodeSnapshotRaw(raw json.RawMessage, cipher dataCipher) (*session.Session, error) {
	if cipher != nil {
		var env encryptedEnvelope
		if err := json.Unmarshal(raw, &env); err == nil && env.Enc != "" {
			data, err := base64.StdEncoding.DecodeString(env.Enc)
			if err != nil {
				return nil, err
			}
			plain, err := cipher.Decrypt(data)
			if err != nil {
				return nil, err
			}
			var sess session.Session
			if err := json.Unmarshal(plain, &sess); err != nil {
				return nil, err
			}
			return &sess, nil
		}
	} else {
		var env encryptedEnvelope
		if err := json.Unmarshal(raw, &env); err == nil && env.Enc != "" {
			return nil, fmt.Errorf("snapshot: encrypted record requires encryption key")
		}
	}
	var sess session.Session
	if err := json.Unmarshal(raw, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}
