package persistence

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/yndnr/tokmesh-go/internal/session"
)

const (
	walEventUpsert = "upsert"
	walEventDelete = "delete"
)

type walEvent struct {
	Type    string           `json:"type"`
	Session *session.Session `json:"session,omitempty"`
	ID      string           `json:"id,omitempty"`
}

type wal struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	writer *bufio.Writer
	cipher dataCipher
}

func openWAL(path string, cipher dataCipher) (*wal, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &wal{
		path:   path,
		file:   file,
		writer: bufio.NewWriter(file),
		cipher: cipher,
	}, nil
}

// AppendUpsert 记录 Session upsert 事件，供恢复时重建状态。
func (w *wal) AppendUpsert(sess *session.Session) error {
	copy := *sess
	return w.append(walEvent{Type: walEventUpsert, Session: &copy})
}

// AppendDelete 记录 Session 删除事件。
func (w *wal) AppendDelete(id string) error {
	return w.append(walEvent{Type: walEventDelete, ID: id})
}

func (w *wal) append(event walEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer == nil {
		return errors.New("wal closed")
	}
	if err := encodeWALEvent(w.writer, event, w.cipher); err != nil {
		return err
	}
	return w.writer.Flush()
}

// Replay 依次回放 WAL 事件，将状态注入内存 store。
func (w *wal) Replay(store *session.Store) error {
	file, err := os.Open(w.path)
	if err != nil {
		return err
	}
	defer file.Close()
	return decodeWALEvents(file, w.cipher, store)
}

func encodeWALEvent(w io.Writer, event walEvent, cipher dataCipher) error {
	enc := json.NewEncoder(w)
	if cipher == nil {
		return enc.Encode(event)
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	ciphertext, err := cipher.Encrypt(payload)
	if err != nil {
		return err
	}
	record := encryptedEnvelope{Enc: base64.StdEncoding.EncodeToString(ciphertext)}
	return enc.Encode(record)
}

func decodeWALEvents(r io.Reader, cipher dataCipher, store *session.Store) error {
	dec := json.NewDecoder(r)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		event, err := decodeWalRaw(raw, cipher)
		if err != nil {
			return err
		}
		switch event.Type {
		case walEventUpsert:
			if event.Session != nil {
				copy := *event.Session
				store.PutSession(&copy)
			}
		case walEventDelete:
			if event.ID != "" {
				store.DeleteSession(event.ID)
			}
		}
	}
}

func decodeWalRaw(raw json.RawMessage, cipher dataCipher) (*walEvent, error) {
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
			var event walEvent
			if err := json.Unmarshal(plain, &event); err != nil {
				return nil, err
			}
			return &event, nil
		}
	} else {
		var env encryptedEnvelope
		if err := json.Unmarshal(raw, &env); err == nil && env.Enc != "" {
			return nil, fmt.Errorf("wal: encrypted record requires encryption key")
		}
	}
	var event walEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Truncate 清空 WAL 文件并重置缓冲区，快照成功后调用。
func (w *wal) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer == nil || w.file == nil {
		return nil
	}
	if err := w.writer.Flush(); err != nil {
		return err
	}
	if err := w.file.Truncate(0); err != nil {
		return err
	}
	if _, err := w.file.Seek(0, 0); err != nil {
		return err
	}
	w.writer = bufio.NewWriter(w.file)
	return nil
}

// Close 刷新缓冲并关闭底层文件句柄。
func (w *wal) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		_ = w.writer.Flush()
	}
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		w.writer = nil
		return err
	}
	return nil
}
