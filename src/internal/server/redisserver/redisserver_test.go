// Package redisserver provides a Redis protocol compatible server.
package redisserver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// ============================================================
// Mock repositories for testing
// ============================================================

// mockSessionRepo implements service.SessionRepository for testing
type mockSessionRepo struct {
	sessions map[string]*domain.Session
	mu       sync.RWMutex
}

func newMockSessionRepo() *mockSessionRepo {
	return &mockSessionRepo{
		sessions: make(map[string]*domain.Session),
	}
}

func (r *mockSessionRepo) Create(ctx context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}

func (r *mockSessionRepo) Get(ctx context.Context, id string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if session, ok := r.sessions[id]; ok {
		return session, nil
	}
	return nil, domain.ErrSessionNotFound
}

func (r *mockSessionRepo) Update(ctx context.Context, session *domain.Session, expectedVersion uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[session.ID]; !ok {
		return domain.ErrSessionNotFound
	}
	r.sessions[session.ID] = session
	return nil
}

func (r *mockSessionRepo) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[id]; !ok {
		return domain.ErrSessionNotFound
	}
	delete(r.sessions, id)
	return nil
}

func (r *mockSessionRepo) List(ctx context.Context, filter *service.SessionFilter) ([]*domain.Session, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Session, 0)
	for _, s := range r.sessions {
		result = append(result, s)
	}
	return result, len(result), nil
}

func (r *mockSessionRepo) CountByUserID(ctx context.Context, userID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, s := range r.sessions {
		if s.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (r *mockSessionRepo) ListByUserID(ctx context.Context, userID string) ([]*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.Session, 0)
	for _, s := range r.sessions {
		if s.UserID == userID {
			result = append(result, s)
		}
	}
	return result, nil
}

func (r *mockSessionRepo) DeleteByUserID(ctx context.Context, userID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for id, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, id)
			count++
		}
	}
	return count, nil
}

func (r *mockSessionRepo) DeleteExpired(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	now := time.Now().UnixMilli()
	for id, s := range r.sessions {
		if s.ExpiresAt < now {
			delete(r.sessions, id)
			count++
		}
	}
	return count, nil
}

// mockTokenRepo implements service.TokenRepository for testing
type mockTokenRepo struct {
	sessions map[string]*domain.Session // tokenHash -> session
	mu       sync.RWMutex
}

func newMockTokenRepo() *mockTokenRepo {
	return &mockTokenRepo{
		sessions: make(map[string]*domain.Session),
	}
}

func (r *mockTokenRepo) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if session, ok := r.sessions[tokenHash]; ok {
		return session, nil
	}
	return nil, domain.ErrTokenInvalid
}

func (r *mockTokenRepo) UpdateSession(ctx context.Context, session *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.TokenHash] = session
	return nil
}

// mockAPIKeyRepo implements service.APIKeyRepository for testing
type mockAPIKeyRepo struct {
	keys map[string]*domain.APIKey
	mu   sync.RWMutex
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{
		keys: make(map[string]*domain.APIKey),
	}
}

func (r *mockAPIKeyRepo) Create(ctx context.Context, key *domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.KeyID] = key
	return nil
}

func (r *mockAPIKeyRepo) Get(ctx context.Context, keyID string) (*domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if key, ok := r.keys[keyID]; ok {
		return key, nil
	}
	return nil, domain.ErrAPIKeyNotFound
}

func (r *mockAPIKeyRepo) Update(ctx context.Context, key *domain.APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys[key.KeyID] = key
	return nil
}

func (r *mockAPIKeyRepo) Delete(ctx context.Context, keyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.keys, keyID)
	return nil
}

func (r *mockAPIKeyRepo) List(ctx context.Context) ([]*domain.APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*domain.APIKey, 0, len(r.keys))
	for _, k := range r.keys {
		result = append(result, k)
	}
	return result, nil
}

// Helper function to create test services
func newTestServices() (*service.SessionService, *service.TokenService, *service.AuthService) {
	sessionRepo := newMockSessionRepo()
	tokenRepo := newMockTokenRepo()
	apiKeyRepo := newMockAPIKeyRepo()

	tokenSvc := service.NewTokenService(tokenRepo, nil)
	sessionSvc := service.NewSessionService(sessionRepo, tokenSvc)
	authSvc := service.NewAuthService(apiKeyRepo, nil)

	return sessionSvc, tokenSvc, authSvc
}

// ============================================================
// Server tests
// ============================================================

func TestServer_New(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	srv := New(nil, sessionSvc, tokenSvc, authSvc, nil)
	if srv == nil {
		t.Fatal("New() returned nil")
	}
	if srv.cfg == nil {
		t.Error("cfg should not be nil")
	}
	if srv.handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestServer_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PlainEnabled {
		t.Error("PlainEnabled should be false by default")
	}
	if cfg.PlainAddress != "127.0.0.1:6379" {
		t.Errorf("PlainAddress = %q, want %q", cfg.PlainAddress, "127.0.0.1:6379")
	}
	if cfg.TLSEnabled {
		t.Error("TLSEnabled should be false by default")
	}
	if cfg.TLSAddress != "127.0.0.1:6380" {
		t.Errorf("TLSAddress = %q, want %q", cfg.TLSAddress, "127.0.0.1:6380")
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, 30*time.Second)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, 30*time.Second)
	}
	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 5*time.Minute)
	}
}

func TestServer_Shutdown(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	// Test shutdown of server that was never started
	srv := New(nil, sessionSvc, tokenSvc, authSvc, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}

func TestServer_Start_Disabled(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	// Both plain and TLS disabled - should return immediately
	cfg := &Config{
		PlainEnabled: false,
		TLSEnabled:   false,
	}
	srv := New(cfg, sessionSvc, tokenSvc, authSvc, nil)

	if err := srv.Start(context.Background()); err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}
}

// ============================================================
// CommandHandler permission tests
// ============================================================

func TestCommandHandler_CheckPermission(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()
	srv := New(nil, sessionSvc, tokenSvc, authSvc, nil)
	h := srv.handler

	tests := []struct {
		name    string
		role    string
		command string
		allowed bool
	}{
		// Admin has full access
		{"admin GET", "admin", "GET", true},
		{"admin SET", "admin", "SET", true},
		{"admin DEL", "admin", "DEL", true},
		{"admin TM.CREATE", "admin", "TM.CREATE", true},
		{"admin TM.VALIDATE", "admin", "TM.VALIDATE", true},

		// Issuer can read and write
		{"issuer GET", "issuer", "GET", true},
		{"issuer SET", "issuer", "SET", true},
		{"issuer DEL", "issuer", "DEL", true},
		{"issuer TM.CREATE", "issuer", "TM.CREATE", true},
		{"issuer TM.VALIDATE", "issuer", "TM.VALIDATE", true},

		// Validator can only read
		{"validator GET", "validator", "GET", true},
		{"validator SET", "validator", "SET", false},
		{"validator DEL", "validator", "DEL", false},
		{"validator TM.CREATE", "validator", "TM.CREATE", false},
		{"validator TM.VALIDATE", "validator", "TM.VALIDATE", true},
		{"validator TTL", "validator", "TTL", true},
		{"validator EXISTS", "validator", "EXISTS", true},
		{"validator SCAN", "validator", "SCAN", true},

		// Metrics role has no access to session commands
		{"metrics GET", "metrics", "GET", false},
		{"metrics SET", "metrics", "SET", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &ConnState{
				Authenticated: true,
				APIKey: &service.APIKeyInfo{
					Role:    tt.role,
					Enabled: true,
				},
			}
			got := h.checkPermission(state, tt.command)
			if got != tt.allowed {
				t.Errorf("checkPermission(role=%q, cmd=%q) = %v, want %v",
					tt.role, tt.command, got, tt.allowed)
			}
		})
	}
}

// ============================================================
// ConnState tests
// ============================================================

func TestConnState(t *testing.T) {
	state := &ConnState{
		Authenticated: false,
		APIKey:        nil,
	}

	if state.Authenticated {
		t.Error("Authenticated should be false initially")
	}

	state.Authenticated = true
	state.APIKey = &service.APIKeyInfo{
		KeyID: "test-key",
		Role:  "admin",
	}

	if !state.Authenticated {
		t.Error("Authenticated should be true")
	}
	if state.APIKey.KeyID != "test-key" {
		t.Errorf("APIKey.KeyID = %q, want %q", state.APIKey.KeyID, "test-key")
	}
}

// ============================================================
// Config tests
// ============================================================

func TestConfig_CustomValues(t *testing.T) {
	cfg := &Config{
		PlainEnabled: true,
		PlainAddress: "0.0.0.0:16379",
		TLSEnabled:   true,
		TLSAddress:   "0.0.0.0:16380",
	}

	if !cfg.PlainEnabled {
		t.Error("PlainEnabled should be true")
	}
	if cfg.PlainAddress != "0.0.0.0:16379" {
		t.Errorf("PlainAddress = %q, want %q", cfg.PlainAddress, "0.0.0.0:16379")
	}
	if !cfg.TLSEnabled {
		t.Error("TLSEnabled should be true")
	}
	if cfg.TLSAddress != "0.0.0.0:16380" {
		t.Errorf("TLSAddress = %q, want %q", cfg.TLSAddress, "0.0.0.0:16380")
	}
}

// ============================================================
// Connection tests
// ============================================================

func TestConn_NewAndClose(t *testing.T) {
	// Create a pair of connected net.Conn using net.Pipe
	server, client := net.Pipe()
	defer client.Close()

	// Test newConn
	conn := newConn(server)
	if conn == nil {
		t.Fatal("newConn returned nil")
	}
	if conn.netConn != server {
		t.Error("netConn not set correctly")
	}
	if conn.br == nil {
		t.Error("bufio.Reader not initialized")
	}
	if conn.bw == nil {
		t.Error("bufio.Writer not initialized")
	}

	// Test initial state
	state := conn.GetState()
	if state.Authenticated {
		t.Error("should not be authenticated initially")
	}
	if state.APIKey != nil {
		t.Error("APIKey should be nil initially")
	}

	// Test Close
	if err := conn.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Double close should not error
	if err := conn.Close(); err != nil {
		t.Errorf("double Close() error = %v", err)
	}
}

func TestConn_SetAndGetState(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := newConn(server)

	// Set authenticated state
	newState := ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    "admin",
			Enabled: true,
		},
	}
	conn.SetState(newState)

	// Get state and verify
	gotState := conn.GetState()
	if !gotState.Authenticated {
		t.Error("Authenticated should be true")
	}
	if gotState.APIKey == nil {
		t.Fatal("APIKey should not be nil")
	}
	if gotState.APIKey.KeyID != "test-key-id" {
		t.Errorf("KeyID = %q, want %q", gotState.APIKey.KeyID, "test-key-id")
	}
}

func TestConn_RemoteAddr(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()

	conn := newConn(server)
	addr := conn.RemoteAddr()

	// net.Pipe returns pipe connections with specific addresses
	if addr == nil {
		t.Error("RemoteAddr() returned nil")
	}
}

// ============================================================
// Server integration tests
// ============================================================

func TestServer_StartPlain(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	// Use a random available port
	cfg := &Config{
		PlainEnabled: true,
		PlainAddress: "127.0.0.1:0", // 0 means pick an available port
		TLSEnabled:   false,
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		IdleTimeout:  time.Second,
	}
	srv := New(cfg, sessionSvc, tokenSvc, authSvc, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Wait a bit for server to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestServer_ServeConn_Ping(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	cfg := &Config{
		PlainEnabled: true,
		PlainAddress: "127.0.0.1:0",
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		IdleTimeout:  time.Second,
	}
	srv := New(cfg, sessionSvc, tokenSvc, authSvc, nil)

	// Create a connected pair
	server, client := net.Pipe()
	defer client.Close()

	// Start serving the connection in background
	go func() {
		conn := newConn(server)
		srv.serveConn(context.Background(), conn)
	}()

	// Send PING command
	_, err := client.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Read response
	buf := make([]byte, 100)
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	response := string(buf[:n])
	if response != "+PONG\r\n" {
		t.Errorf("PING response = %q, want +PONG\\r\\n", response)
	}

	// Send QUIT
	_, err = client.Write([]byte("*1\r\n$4\r\nQUIT\r\n"))
	if err != nil {
		t.Fatalf("Write QUIT error: %v", err)
	}

	// Read OK response
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, _ = client.Read(buf)
	response = string(buf[:n])
	if response != "+OK\r\n" {
		t.Errorf("QUIT response = %q, want +OK\\r\\n", response)
	}
}

func TestServer_ServeConn_Auth(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	// Create an API key for testing
	ctx := context.Background()
	createResp, err := authSvc.CreateAPIKey(ctx, &service.CreateAPIKeyRequest{
		Name: "test-key",
		Role: "admin",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey error: %v", err)
	}

	cfg := &Config{
		PlainEnabled: true,
		PlainAddress: "127.0.0.1:0",
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		IdleTimeout:  time.Second,
	}
	srv := New(cfg, sessionSvc, tokenSvc, authSvc, nil)

	server, client := net.Pipe()
	defer client.Close()

	go func() {
		conn := newConn(server)
		srv.serveConn(context.Background(), conn)
	}()

	// Send AUTH command with valid credentials
	authCmd := fmt.Sprintf("*3\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
		len(createResp.KeyID), createResp.KeyID,
		len(createResp.Secret), createResp.Secret)
	_, err = client.Write([]byte(authCmd))
	if err != nil {
		t.Fatalf("Write AUTH error: %v", err)
	}

	buf := make([]byte, 100)
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("Read AUTH response error: %v", err)
	}

	response := string(buf[:n])
	if response != "+OK\r\n" {
		t.Errorf("AUTH response = %q, want +OK\\r\\n", response)
	}
}

func TestServer_ServeConn_ProtocolError(t *testing.T) {
	sessionSvc, tokenSvc, authSvc := newTestServices()

	cfg := &Config{
		PlainEnabled: true,
		PlainAddress: "127.0.0.1:0",
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
		IdleTimeout:  time.Second,
	}
	srv := New(cfg, sessionSvc, tokenSvc, authSvc, nil)

	server, client := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		conn := newConn(server)
		srv.serveConn(context.Background(), conn)
		close(done)
	}()

	// Send invalid protocol (exceeds limit)
	invalidCmd := "*10000\r\n" // Array size exceeds MaxArrayLen
	_, err := client.Write([]byte(invalidCmd))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	buf := make([]byte, 200)
	client.SetReadDeadline(time.Now().Add(time.Second))
	n, _ := client.Read(buf)

	response := string(buf[:n])
	if !strings.Contains(response, "ERR") {
		t.Errorf("Expected error response, got %q", response)
	}

	// Connection should be closed after protocol error
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("Connection not closed after protocol error")
	}
}
