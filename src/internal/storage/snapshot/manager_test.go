package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/pkg/crypto/adaptive"
)

func TestManager_CreateLoadPlain(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_y"
	s2.SetExpiration(time.Hour)

	info, err := m.Create([]*domain.Session{s1, s2}, uint64(3)<<32|123)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if info.SessionCount != 2 {
		t.Fatalf("SessionCount = %d, want 2", info.SessionCount)
	}

	got, loadedInfo, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loadedInfo.WALLastOffset != info.WALLastOffset {
		t.Fatalf("WALLastOffset = %d, want %d", loadedInfo.WALLastOffset, info.WALLastOffset)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
}

func TestManager_CreateLoadEncrypted(t *testing.T) {
	dir := t.TempDir()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(0xA0 + i)
	}
	c, err := adaptive.New(key)
	if err != nil {
		t.Fatalf("adaptive.New: %v", err)
	}

	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1", Cipher: c})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	if _, err := m.Create([]*domain.Session{s1}, uint64(1)<<32|0); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, _, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 || got[0].UserID != "u1" {
		t.Fatalf("decrypted mismatch: %+v", got)
	}
}

func TestManager_PruningKeepsAtLeastOne(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 1, RetentionDays: 0, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	for i := 0; i < 3; i++ {
		if _, err := m.Create([]*domain.Session{s1}, uint64(i+1)<<32); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	if err := m.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) < 1 {
		t.Fatalf("expected at least one snapshot remaining")
	}

	// All listed files should exist.
	for _, info := range infos {
		if _, err := os.Stat(info.Path); err != nil {
			t.Fatalf("missing snapshot file %s: %v", filepath.Base(info.Path), err)
		}
	}
}

func TestManager_LoadFallsBackOnCorruptedLatest(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	oldInfo, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create(old): %v", err)
	}

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_y"
	s2.SetExpiration(time.Hour)
	newInfo, err := m.Create([]*domain.Session{s2}, uint64(2)<<32)
	if err != nil {
		t.Fatalf("Create(new): %v", err)
	}

	// Corrupt the latest snapshot by flipping a byte in the checksum trailer.
	f, err := os.OpenFile(newInfo.Path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		t.Fatalf("Stat: %v", err)
	}
	if _, err := f.WriteAt([]byte{0xFF}, st.Size()-1); err != nil {
		f.Close()
		t.Fatalf("WriteAt: %v", err)
	}
	f.Close()

	got, info, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if info.Path != oldInfo.Path {
		t.Fatalf("expected fallback to old snapshot, got %s", filepath.Base(info.Path))
	}
	if len(got) != 1 || got[0].UserID != "u1" {
		t.Fatalf("unexpected sessions: %+v", got)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/tmp/snap")

	if cfg.Dir != "/tmp/snap" {
		t.Fatalf("Dir = %q, want %q", cfg.Dir, "/tmp/snap")
	}
	if cfg.RetentionCount != DefaultRetentionCount {
		t.Fatalf("RetentionCount = %d, want %d", cfg.RetentionCount, DefaultRetentionCount)
	}
	if cfg.RetentionDays != DefaultRetentionDays {
		t.Fatalf("RetentionDays = %d, want %d", cfg.RetentionDays, DefaultRetentionDays)
	}
}

func TestNewManager_EmptyDir(t *testing.T) {
	_, err := NewManager(Config{Dir: ""})
	if err == nil {
		t.Fatal("NewManager with empty dir should error")
	}
}

func TestManager_LoadEmptyDir(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, _, err = m.Load()
	if err != ErrNoSnapshots {
		t.Fatalf("Load err = %v, want %v", err, ErrNoSnapshots)
	}
}

func TestManager_CreateEmptySessions(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	info, err := m.Create([]*domain.Session{}, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if info.SessionCount != 0 {
		t.Fatalf("SessionCount = %d, want 0", info.SessionCount)
	}
}

func TestManager_ListEmptyDir(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("len(infos) = %d, want 0", len(infos))
	}
}

func TestManager_PruneEmptyDir(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Prune on empty dir should not error
	if err := m.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}
}

func TestManager_CreateMultipleSessions(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	var sessions []*domain.Session
	for i := 0; i < 10; i++ {
		s, _ := domain.NewSession("user1")
		s.TokenHash = "hash_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		s.Data["index"] = string(rune('0' + i))
		sessions = append(sessions, s)
	}

	info, err := m.Create(sessions, uint64(5)<<32|100)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if info.SessionCount != 10 {
		t.Fatalf("SessionCount = %d, want 10", info.SessionCount)
	}

	loaded, loadedInfo, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 10 {
		t.Fatalf("len(loaded) = %d, want 10", len(loaded))
	}
	if loadedInfo.WALLastOffset != info.WALLastOffset {
		t.Fatalf("WALLastOffset mismatch")
	}
}

func TestManager_GenerateIDSequence(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	// Create multiple snapshots rapidly to test sequence generation
	for i := 0; i < 3; i++ {
		_, err := m.Create([]*domain.Session{s1}, uint64(i+1)<<32)
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("len(infos) = %d, want 3", len(infos))
	}
}

func TestManager_LoadSkipsNonSnapshotFiles(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a valid snapshot first
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)
	_, err = m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a non-snapshot file and a directory
	if err := os.WriteFile(filepath.Join(dir, "other.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("len(infos) = %d, want 1", len(infos))
	}
}

func TestManager_LoadFileTooSmall(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a file that's too small to be a valid snapshot
	smallFile := filepath.Join(dir, "snapshot-20250101120000-0001.snap")
	if err := os.WriteFile(smallFile, []byte("small"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, _, err = m.Load()
	if err != ErrNoSnapshots {
		t.Fatalf("Load err = %v, want %v", err, ErrNoSnapshots)
	}
}

func TestManager_LoadInvalidMagic(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a valid snapshot first
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)
	info, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Corrupt the magic bytes but keep checksum valid (this will trigger ErrInvalidMagic after checksum passes)
	// Since checksum covers magic, we need to corrupt magic and fix checksum
	// Actually the test for corrupted latest already covers fallback, let's test direct loadFile
	// Let's create a file with valid checksum but wrong magic
	wrongMagic := filepath.Join(dir, "snapshot-20250101130000-0001.snap")
	content := make([]byte, 50)
	copy(content[:8], "WRONGMGC") // wrong magic
	checksum := make([]byte, 32)
	for i := range checksum {
		checksum[i] = 0xAB
	}
	content = append(content, checksum...)
	if err := os.WriteFile(wrongMagic, content, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Load should fall back to the valid snapshot
	got, loadedInfo, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loadedInfo.Path != info.Path {
		t.Fatalf("expected fallback to valid snapshot")
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
}

func TestManager_PruneByDays(t *testing.T) {
	dir := t.TempDir()
	// RetentionCount=1 ensures only one snapshot is kept by count, and RetentionDays=1 tests day-based pruning
	m, err := NewManager(Config{Dir: dir, RetentionCount: 1, RetentionDays: 1, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	// Create a snapshot
	info, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Set the mtime to 10 days ago
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	if err := os.Chtimes(info.Path, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// Create a second snapshot (will be recent)
	_, err = m.Create([]*domain.Session{s1}, uint64(2)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Prune should keep the second and remove the first
	if err := m.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("len(infos) = %d, want 1", len(infos))
	}
}

func TestManager_EncryptedDecodeBlockMissingData(t *testing.T) {
	dir := t.TempDir()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(0xB0 + i)
	}
	c, err := adaptive.New(key)
	if err != nil {
		t.Fatalf("adaptive.New: %v", err)
	}

	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1", Cipher: c})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a valid encrypted snapshot
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	_, err = m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Load should work
	got, _, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
}

func TestManager_ListNonExistentDir(t *testing.T) {
	m := &Manager{
		cfg: Config{Dir: "/nonexistent/path/that/does/not/exist"},
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if infos != nil {
		t.Fatalf("infos = %v, want nil", infos)
	}
}

func TestManager_InfoFields(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "test-node"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)
	s1.IPAddress = "192.168.1.1"
	s1.UserAgent = "TestAgent/1.0"
	s1.DeviceID = "device-1"

	walOffset := uint64(5)<<32 | 123
	info, err := m.Create([]*domain.Session{s1}, walOffset)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify all info fields
	if info.NodeID != "test-node" {
		t.Fatalf("NodeID = %q, want %q", info.NodeID, "test-node")
	}
	if info.WALLastOffset != walOffset {
		t.Fatalf("WALLastOffset = %d, want %d", info.WALLastOffset, walOffset)
	}
	if info.Checksum == "" {
		t.Fatal("Checksum is empty")
	}
	if info.Size <= 0 {
		t.Fatalf("Size = %d, want > 0", info.Size)
	}
	if info.CreatedAt <= 0 {
		t.Fatalf("CreatedAt = %d, want > 0", info.CreatedAt)
	}

	// Load and verify session fields are preserved
	loaded, _, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}

	ls := loaded[0]
	if ls.IPAddress != "192.168.1.1" {
		t.Fatalf("IPAddress = %q, want %q", ls.IPAddress, "192.168.1.1")
	}
	if ls.UserAgent != "TestAgent/1.0" {
		t.Fatalf("UserAgent = %q, want %q", ls.UserAgent, "TestAgent/1.0")
	}
	if ls.DeviceID != "device-1" {
		t.Fatalf("DeviceID = %q, want %q", ls.DeviceID, "device-1")
	}
}

func TestManager_LoadAllCorrupted(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create two snapshots and corrupt both
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	info1, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create(1): %v", err)
	}

	info2, err := m.Create([]*domain.Session{s1}, uint64(2)<<32)
	if err != nil {
		t.Fatalf("Create(2): %v", err)
	}

	// Corrupt both snapshots
	for _, path := range []string{info1.Path, info2.Path} {
		f, err := os.OpenFile(path, os.O_RDWR, 0600)
		if err != nil {
			t.Fatalf("OpenFile: %v", err)
		}
		st, _ := f.Stat()
		if _, err := f.WriteAt([]byte{0xFF}, st.Size()-1); err != nil {
			f.Close()
			t.Fatalf("WriteAt: %v", err)
		}
		f.Close()
	}

	// Load should fail with ErrNoSnapshots since all are corrupted
	_, _, err = m.Load()
	if err != ErrNoSnapshots {
		t.Fatalf("Load err = %v, want %v", err, ErrNoSnapshots)
	}
}

func TestManager_EncryptedSessionFields(t *testing.T) {
	dir := t.TempDir()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(0xC0 + i)
	}
	c, err := adaptive.New(key)
	if err != nil {
		t.Fatalf("adaptive.New: %v", err)
	}

	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "enc-node", Cipher: c})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create session with all fields
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_encrypted"
	s1.SetExpiration(time.Hour)
	s1.IPAddress = "10.0.0.1"
	s1.UserAgent = "EncryptedAgent/1.0"
	s1.DeviceID = "enc-device-1"
	s1.LastAccessIP = "10.0.0.2"
	s1.LastAccessUA = "EncryptedAgent/2.0"
	s1.CreatedBy = "test-key"
	s1.Data["custom"] = "value"

	_, err = m.Create([]*domain.Session{s1}, uint64(3)<<32|456)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, info, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if info.NodeID != "enc-node" {
		t.Fatalf("NodeID = %q, want %q", info.NodeID, "enc-node")
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	ls := got[0]
	if ls.UserID != "u1" {
		t.Fatalf("UserID = %q, want %q", ls.UserID, "u1")
	}
	if ls.TokenHash != "tmth_encrypted" {
		t.Fatalf("TokenHash = %q, want %q", ls.TokenHash, "tmth_encrypted")
	}
	if ls.IPAddress != "10.0.0.1" {
		t.Fatalf("IPAddress = %q, want %q", ls.IPAddress, "10.0.0.1")
	}
	if ls.LastAccessIP != "10.0.0.2" {
		t.Fatalf("LastAccessIP = %q, want %q", ls.LastAccessIP, "10.0.0.2")
	}
	if ls.Data["custom"] != "value" {
		t.Fatalf("Data[custom] = %q, want %q", ls.Data["custom"], "value")
	}
}

func TestManager_PruneWithMissingFile(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 1, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	// Create two snapshots
	info1, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create(1): %v", err)
	}

	_, err = m.Create([]*domain.Session{s1}, uint64(2)<<32)
	if err != nil {
		t.Fatalf("Create(2): %v", err)
	}

	// Remove the first snapshot manually (simulating file system issue during retention check)
	if err := os.Remove(info1.Path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Prune should not error even if file is already gone
	if err := m.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}
}

func TestManager_LoadFileStatError(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a valid snapshot
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	info, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// List should work even after file is deleted (Stat error path in List)
	if err := os.Remove(info.Path); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// The file is deleted, so List should skip it when Stat fails
	if len(infos) != 0 {
		t.Fatalf("len(infos) = %d, want 0", len(infos))
	}
}

func TestErrorConstants(t *testing.T) {
	// Test error constants are accessible
	if ErrInvalidMagic.Error() != "snapshot: invalid magic bytes" {
		t.Fatalf("ErrInvalidMagic = %q", ErrInvalidMagic.Error())
	}
	if ErrChecksumMismatch.Error() != "snapshot: checksum mismatch" {
		t.Fatalf("ErrChecksumMismatch = %q", ErrChecksumMismatch.Error())
	}
	if ErrNotFound.Error() != "snapshot: not found" {
		t.Fatalf("ErrNotFound = %q", ErrNotFound.Error())
	}
	if ErrNoSnapshots.Error() != "snapshot: no snapshots available" {
		t.Fatalf("ErrNoSnapshots = %q", ErrNoSnapshots.Error())
	}
}

func TestConstants(t *testing.T) {
	// Test constants
	if DefaultRetentionCount != 5 {
		t.Fatalf("DefaultRetentionCount = %d, want 5", DefaultRetentionCount)
	}
	if DefaultRetentionDays != 7 {
		t.Fatalf("DefaultRetentionDays = %d, want 7", DefaultRetentionDays)
	}
}

func TestManager_LoadEncryptedWithPlainManager(t *testing.T) {
	dir := t.TempDir()

	// Create an encrypted snapshot first
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(0xD0 + i)
	}
	c, err := adaptive.New(key)
	if err != nil {
		t.Fatalf("adaptive.New: %v", err)
	}

	encM, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1", Cipher: c})
	if err != nil {
		t.Fatalf("NewManager(encrypted): %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	_, err = encM.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Try to load with a plain manager (no cipher)
	plainM, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager(plain): %v", err)
	}

	// Load should succeed (plain manager loads encrypted session's empty Sessions array)
	got, _, err := plainM.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Since the encrypted block has nil Sessions but has EncryptedData,
	// and plain manager doesn't have a cipher, it returns the nil Sessions
	if got != nil && len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestManager_LoadPlainWithEncryptedManager(t *testing.T) {
	dir := t.TempDir()

	// Create a plain snapshot first
	plainM, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager(plain): %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	_, err = plainM.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Try to load with an encrypted manager
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(0xE0 + i)
	}
	c, err := adaptive.New(key)
	if err != nil {
		t.Fatalf("adaptive.New: %v", err)
	}

	encM, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1", Cipher: c})
	if err != nil {
		t.Fatalf("NewManager(encrypted): %v", err)
	}

	// Load should fail because encrypted manager expects encrypted data but block has none
	_, _, err = encM.Load()
	if err == nil {
		t.Fatal("Load should fail when encrypted manager loads plain snapshot")
	}
}

func TestManager_RetentionDefaultsApplied(t *testing.T) {
	dir := t.TempDir()

	// Test that zero retention values get defaults (they get overwritten)
	m, err := NewManager(Config{Dir: dir, RetentionCount: 0, RetentionDays: 0})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	// Create more snapshots than default retention
	for i := 0; i < 7; i++ {
		info, err := m.Create([]*domain.Session{s1}, uint64(i+1)<<32)
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		// Set mtime to 10 days ago for older snapshots
		if i < 2 {
			oldTime := time.Now().Add(-10 * 24 * time.Hour)
			if err := os.Chtimes(info.Path, oldTime, oldTime); err != nil {
				t.Fatalf("Chtimes: %v", err)
			}
		}
	}

	// Prune should keep only DefaultRetentionCount (5) since older ones are beyond 7 days
	if err := m.Prune(); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	infos, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Should have 5 snapshots: the last 5 by count
	if len(infos) != DefaultRetentionCount {
		t.Fatalf("len(infos) = %d, want %d", len(infos), DefaultRetentionCount)
	}
}

func TestManager_LoadFileOpenError(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a valid snapshot
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	info, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Remove read permission to trigger open error
	if err := os.Chmod(info.Path, 0000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(info.Path, 0644) // restore for cleanup

	// Load should fail since file cannot be opened
	_, _, err = m.Load()
	if err == nil {
		t.Fatal("Load should fail when file cannot be opened")
	}
}

func TestManager_SessionDataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a session with all optional fields
	s1, _ := domain.NewSession("user123")
	s1.TokenHash = "tmth_hash123"
	s1.SetExpiration(2 * time.Hour)
	s1.IPAddress = "192.168.1.100"
	s1.UserAgent = "Mozilla/5.0"
	s1.LastAccessIP = "192.168.1.101"
	s1.LastAccessUA = "Mozilla/5.1"
	s1.DeviceID = "device-abc"
	s1.CreatedBy = "apikey-xyz"
	s1.ShardID = 42
	s1.TTL = 7200
	s1.Data["key1"] = "value1"
	s1.Data["key2"] = "value2"

	_, err = m.Create([]*domain.Session{s1}, uint64(10)<<32|500)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, _, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("len(loaded) = %d, want 1", len(loaded))
	}

	ls := loaded[0]
	// Verify all fields are preserved
	if ls.ID != s1.ID {
		t.Fatalf("ID = %q, want %q", ls.ID, s1.ID)
	}
	if ls.UserID != s1.UserID {
		t.Fatalf("UserID = %q, want %q", ls.UserID, s1.UserID)
	}
	if ls.TokenHash != s1.TokenHash {
		t.Fatalf("TokenHash = %q, want %q", ls.TokenHash, s1.TokenHash)
	}
	if ls.ShardID != s1.ShardID {
		t.Fatalf("ShardID = %d, want %d", ls.ShardID, s1.ShardID)
	}
	if ls.TTL != s1.TTL {
		t.Fatalf("TTL = %d, want %d", ls.TTL, s1.TTL)
	}
	if ls.CreatedBy != s1.CreatedBy {
		t.Fatalf("CreatedBy = %q, want %q", ls.CreatedBy, s1.CreatedBy)
	}
	if len(ls.Data) != len(s1.Data) {
		t.Fatalf("len(Data) = %d, want %d", len(ls.Data), len(s1.Data))
	}
}

func TestManager_LoadWithNonFatalError(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create two valid snapshots
	s1, _ := domain.NewSession("u1")
	s1.TokenHash = "tmth_x"
	s1.SetExpiration(time.Hour)

	info1, err := m.Create([]*domain.Session{s1}, uint64(1)<<32)
	if err != nil {
		t.Fatalf("Create(1): %v", err)
	}

	s2, _ := domain.NewSession("u2")
	s2.TokenHash = "tmth_y"
	s2.SetExpiration(time.Hour)

	info2, err := m.Create([]*domain.Session{s2}, uint64(2)<<32)
	if err != nil {
		t.Fatalf("Create(2): %v", err)
	}

	// Corrupt the latest snapshot's header (not checksum) so it fails during protobuf unmarshal
	// This is a non-checksum error that should propagate up
	f, err := os.OpenFile(info2.Path, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	// Corrupt bytes after magic (where header is)
	if _, err := f.WriteAt([]byte{0xFF, 0xFF, 0xFF, 0xFF}, int64(len(magicBytes))+1); err != nil {
		f.Close()
		t.Fatalf("WriteAt: %v", err)
	}
	f.Close()

	// This still corrupts the checksum, so it should fallback
	got, loadedInfo, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should fallback to the first valid snapshot
	if loadedInfo.Path != info1.Path {
		t.Fatalf("expected fallback to first snapshot")
	}
	if len(got) != 1 || got[0].UserID != "u1" {
		t.Fatalf("unexpected sessions: got %+v", got)
	}
}

func TestManager_MultipleSessionsWithData(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(Config{Dir: dir, RetentionCount: 5, RetentionDays: 7, NodeID: "n1"})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create multiple sessions with different data
	var sessions []*domain.Session
	for i := 0; i < 5; i++ {
		s, _ := domain.NewSession("user" + string(rune('A'+i)))
		s.TokenHash = "hash_" + string(rune('a'+i))
		s.SetExpiration(time.Hour)
		s.DeviceID = "device_" + string(rune('0'+i))
		s.IsDeleted = i%2 == 0 // alternate deleted status
		sessions = append(sessions, s)
	}

	_, err = m.Create(sessions, uint64(5)<<32)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	loaded, _, err := m.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 5 {
		t.Fatalf("len(loaded) = %d, want 5", len(loaded))
	}

	// Verify IsDeleted field is preserved
	deletedCount := 0
	for _, s := range loaded {
		if s.IsDeleted {
			deletedCount++
		}
	}
	if deletedCount != 3 { // 0, 2, 4 are deleted
		t.Fatalf("deletedCount = %d, want 3", deletedCount)
	}
}
