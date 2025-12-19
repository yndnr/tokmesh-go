package redisserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// ============================================================
// Test Helper: Create a mock Conn using net.Pipe
// ============================================================

type testConn struct {
	*Conn
	output *bytes.Buffer
	server net.Conn
	client net.Conn
}

func newTestConn() *testConn {
	server, client := net.Pipe()
	output := &bytes.Buffer{}

	tc := &testConn{
		output: output,
		server: server,
		client: client,
	}

	tc.Conn = &Conn{
		netConn: server,
		br:      bufio.NewReader(server),
		bw:      bufio.NewWriter(output),
	}

	return tc
}

func (tc *testConn) Close() {
	tc.server.Close()
	tc.client.Close()
}

func (tc *testConn) FlushAndGetOutput() string {
	tc.bw.Flush()
	return tc.output.String()
}

func (tc *testConn) Reset() {
	tc.output.Reset()
}

// setAuthenticated sets the connection as authenticated with admin role
func (tc *testConn) setAuthenticated() {
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleAdmin),
			Enabled: true,
		},
	})
}

// ============================================================
// Test: formatRedisError
// ============================================================

func TestFormatRedisError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "domain error",
			err:  domain.ErrSessionNotFound,
			want: "ERR TM-SESS-4040 session not found",
		},
		{
			name: "domain error with details",
			err:  domain.ErrSessionValidation.WithDetails("user_id required"),
			want: "ERR TM-SESS-4001 session validation failed",
		},
		{
			name: "token error",
			err:  domain.ErrTokenInvalid,
			want: "ERR TM-TOKN-4010 invalid token",
		},
		{
			name: "auth error",
			err:  domain.ErrAPIKeyInvalid,
			want: "ERR TM-AUTH-4011 invalid api key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRedisError(tt.err)
			if got != tt.want {
				t.Errorf("formatRedisError() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ============================================================
// Test: Handle (main router)
// ============================================================

func TestCommandHandler_Handle_Unauthenticated(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Unauthenticated connection trying GET
	args := [][]byte{[]byte("GET"), []byte("some-session-id")}
	h.Handle(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "NOAUTH") {
		t.Errorf("expected NOAUTH error, got %q", output)
	}
}

func TestCommandHandler_Handle_UnknownCommand(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()
	tc.setAuthenticated()

	args := [][]byte{[]byte("UNKNOWNCMD")}
	h.Handle(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "unknown command") {
		t.Errorf("expected unknown command error, got %q", output)
	}
}

func TestCommandHandler_Handle_EmptyCommand(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{}
	h.Handle(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "no command") {
		t.Errorf("expected no command error, got %q", output)
	}
}

func TestCommandHandler_Handle_PermissionDenied(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Set as validator role (read-only)
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleValidator),
			Enabled: true,
		},
	})

	// Try to SET (write operation)
	args := [][]byte{[]byte("SET"), []byte("session-id"), []byte("{}")}
	h.Handle(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "permission denied") {
		t.Errorf("expected permission denied error for validator trying SET, got %q", output)
	}
}

func TestCommandHandler_Handle_AuthenticatedCommands(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	// Set as admin role
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleAdmin),
			Enabled: true,
		},
	})

	// Test GET through Handle
	args := [][]byte{[]byte("GET"), []byte("tmss-test-session-id")}
	h.Handle(tc.Conn, args)
	output := tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, "$") {
		t.Errorf("GET via Handle: expected bulk string, got %q", output)
	}

	// Reset output buffer
	tc = newTestConn()
	defer tc.Close()
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleAdmin),
			Enabled: true,
		},
	})

	// Test TTL through Handle
	args = [][]byte{[]byte("TTL"), []byte("tmss-test-session-id")}
	h.Handle(tc.Conn, args)
	output = tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, ":") {
		t.Errorf("TTL via Handle: expected integer, got %q", output)
	}

	// Reset output buffer
	tc = newTestConn()
	defer tc.Close()
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleAdmin),
			Enabled: true,
		},
	})

	// Test EXISTS through Handle
	args = [][]byte{[]byte("EXISTS"), []byte("tmss-test-session-id")}
	h.Handle(tc.Conn, args)
	output = tc.FlushAndGetOutput()
	if output != ":1\r\n" {
		t.Errorf("EXISTS via Handle: expected :1, got %q", output)
	}

	// Reset output buffer
	tc = newTestConn()
	defer tc.Close()
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleAdmin),
			Enabled: true,
		},
	})

	// Test SCAN through Handle
	args = [][]byte{[]byte("SCAN"), []byte("0")}
	h.Handle(tc.Conn, args)
	output = tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, "*") {
		t.Errorf("SCAN via Handle: expected array, got %q", output)
	}

	// Reset output buffer
	tc = newTestConn()
	defer tc.Close()
	tc.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   "test-key-id",
			Role:    string(domain.RoleAdmin),
			Enabled: true,
		},
	})

	// Test EXPIRE through Handle
	args = [][]byte{[]byte("EXPIRE"), []byte("tmss-test-session-id"), []byte("3600")}
	h.Handle(tc.Conn, args)
	output = tc.FlushAndGetOutput()
	if output != ":1\r\n" {
		t.Errorf("EXPIRE via Handle: expected :1, got %q", output)
	}
}

// ============================================================
// Test: AUTH command
// ============================================================

func TestCommandHandler_Auth_TwoArgs(t *testing.T) {
	h, authSvc := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	ctx := context.Background()
	createResp, _ := authSvc.CreateAPIKey(ctx, &service.CreateAPIKeyRequest{
		Name: "test-key",
		Role: string(domain.RoleAdmin),
	})

	// Test AUTH with invalid credentials
	args := [][]byte{[]byte("AUTH"), []byte("invalid-id"), []byte("invalid-secret")}
	h.handleAuth(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "ERR") {
		t.Errorf("expected error for invalid credentials, got %q", output)
	}
	if tc.GetState().Authenticated {
		t.Error("should not be authenticated with invalid credentials")
	}

	_ = createResp
}

func TestCommandHandler_Auth_CombinedFormat(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("AUTH"), []byte("invalid-id:invalid-secret")}
	h.handleAuth(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "ERR") {
		t.Errorf("expected error for invalid credentials, got %q", output)
	}
}

func TestCommandHandler_Auth_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("AUTH")}
	h.handleAuth(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Auth_TooManyArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("AUTH"), []byte("a"), []byte("b"), []byte("c"), []byte("d")}
	h.handleAuth(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

// ============================================================
// Test: PING command
// ============================================================

func TestCommandHandler_Ping(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("PING")}
	h.handlePing(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != "+PONG\r\n" {
		t.Errorf("PING response = %q, want +PONG\\r\\n", output)
	}
}

func TestCommandHandler_Ping_WithMessage(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("PING"), []byte("hello")}
	h.handlePing(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != "$5\r\nhello\r\n" {
		t.Errorf("PING response = %q, want $5\\r\\nhello\\r\\n", output)
	}
}

// ============================================================
// Test: QUIT command
// ============================================================

func TestCommandHandler_Quit(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("QUIT")}
	h.handleQuit(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != "+OK\r\n" {
		t.Errorf("QUIT response = %q, want +OK\\r\\n", output)
	}
}

// ============================================================
// Test: GET command
// ============================================================

func TestCommandHandler_Get_NotFound(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()
	tc.setAuthenticated()

	args := [][]byte{[]byte("GET"), []byte("non-existent-session")}
	h.handleGet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Should return null bulk for not found
	if output != "$-1\r\n" {
		t.Errorf("GET not found response = %q, want $-1\\r\\n", output)
	}
}

func TestCommandHandler_Get_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("GET")}
	h.handleGet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Get_Success(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()
	tc.setAuthenticated()

	args := [][]byte{[]byte("GET"), []byte("tmss-test-session-id")}
	h.handleGet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, "$") {
		t.Errorf("expected bulk string response, got %q", output)
	}
	if !strings.Contains(output, "user123") {
		t.Errorf("expected response to contain user_id, got %q", output)
	}
}

// ============================================================
// Test: SET command
// ============================================================

func TestCommandHandler_Set_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SET"), []byte("key")}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Set_InvalidJSON(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SET"), []byte("session-id"), []byte("invalid-json")}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "invalid JSON") {
		t.Errorf("expected invalid JSON error, got %q", output)
	}
}

func TestCommandHandler_Set_CreateWithoutToken(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Creating new session without token should fail
	jsonValue := `{"user_id":"user123","device_id":"device1"}`
	args := [][]byte{[]byte("SET"), []byte("tmss-new-session"), []byte(jsonValue)}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "token is required") {
		t.Errorf("expected token required error, got %q", output)
	}
}

func TestCommandHandler_Set_CreateWithToken(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Creating new session with token should succeed
	// Valid session ID: tmss- + 26-char ULID = 31 chars
	// Valid token: tmtk_ + 43-char base64 = 48 chars
	sessionID := "tmss-01ARZ3NDEKTSV4RRFFQ69G5FAV"
	validToken := "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq" // 48 chars total
	jsonValue := fmt.Sprintf(`{"user_id":"user123","device_id":"device1","token":"%s"}`, validToken)
	args := [][]byte{[]byte("SET"), []byte(sessionID), []byte(jsonValue)}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != "+OK\r\n" {
		t.Errorf("SET create response = %q, want +OK\\r\\n", output)
	}
}

func TestCommandHandler_Set_WithEX(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Valid session ID and token for creating new session with TTL
	sessionID := "tmss-01ARZ3NDEKTSV4RRFFQ69G5FAW"
	validToken := "tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopR" // 48 chars total
	jsonValue := fmt.Sprintf(`{"user_id":"user123","token":"%s"}`, validToken)
	args := [][]byte{[]byte("SET"), []byte(sessionID), []byte(jsonValue), []byte("EX"), []byte("3600")}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != "+OK\r\n" {
		t.Errorf("SET with EX response = %q, want +OK\\r\\n", output)
	}
}

func TestCommandHandler_Set_Update(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	// Update existing session
	jsonValue := `{"user_id":"user123","device_id":"new-device"}`
	args := [][]byte{[]byte("SET"), []byte("tmss-test-session-id"), []byte(jsonValue)}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != "+OK\r\n" {
		t.Errorf("SET update response = %q, want +OK\\r\\n", output)
	}
}

func TestCommandHandler_Set_UpdateWithTokenRejected(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	// Attempt to update existing session with token should be rejected
	jsonValue := `{"user_id":"user123","token":"tmtk_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq"}`
	args := [][]byte{[]byte("SET"), []byte("tmss-test-session-id"), []byte(jsonValue)}
	h.handleSet(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "TM-ARG-4003") {
		t.Errorf("SET update with token should be rejected with TM-ARG-4003, got %q", output)
	}
	if !strings.Contains(output, "token rotation") {
		t.Errorf("error should mention token rotation, got %q", output)
	}
}

// ============================================================
// Test: DEL command
// ============================================================

func TestCommandHandler_Del_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("DEL")}
	h.handleDel(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Del_NotFound(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// DEL is idempotent - deleting non-existent session still "succeeds"
	// This matches the session service's idempotent behavior
	args := [][]byte{[]byte("DEL"), []byte("non-existent")}
	h.handleDel(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Idempotent delete: counts as success
	if output != ":1\r\n" {
		t.Errorf("DEL not found response = %q, want :1\\r\\n (idempotent)", output)
	}
}

func TestCommandHandler_Del_Success(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("DEL"), []byte("tmss-test-session-id")}
	h.handleDel(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":1\r\n" {
		t.Errorf("DEL success response = %q, want :1\\r\\n", output)
	}
}

func TestCommandHandler_Del_Multiple(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	// DEL is idempotent, both deletes count as success
	args := [][]byte{[]byte("DEL"), []byte("tmss-test-session-id"), []byte("non-existent")}
	h.handleDel(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Both deletes succeed due to idempotent behavior
	if output != ":2\r\n" {
		t.Errorf("DEL multiple response = %q, want :2\\r\\n (idempotent)", output)
	}
}

func TestCommandHandler_Del_Limit(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := make([][]byte, 1003)
	args[0] = []byte("DEL")
	for i := 1; i < 1003; i++ {
		args[i] = []byte("key" + string(rune('0'+i%10)))
	}

	h.handleDel(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "TM-ARG-4002") {
		t.Errorf("expected TM-ARG-4002 error for too many keys, got %q", output)
	}
}

// ============================================================
// Test: EXPIRE command
// ============================================================

func TestCommandHandler_Expire_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXPIRE"), []byte("key")}
	h.handleExpire(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Expire_InvalidSeconds(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXPIRE"), []byte("key"), []byte("invalid")}
	h.handleExpire(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "not an integer") {
		t.Errorf("expected integer error, got %q", output)
	}
}

func TestCommandHandler_Expire_NotFound(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXPIRE"), []byte("non-existent"), []byte("3600")}
	h.handleExpire(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":0\r\n" {
		t.Errorf("EXPIRE not found response = %q, want :0\\r\\n", output)
	}
}

func TestCommandHandler_Expire_Success(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXPIRE"), []byte("tmss-test-session-id"), []byte("7200")}
	h.handleExpire(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":1\r\n" {
		t.Errorf("EXPIRE success response = %q, want :1\\r\\n", output)
	}
}

// ============================================================
// Test: TTL command
// ============================================================

func TestCommandHandler_TTL_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TTL")}
	h.handleTTL(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_TTL_NotFound(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TTL"), []byte("non-existent")}
	h.handleTTL(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":-2\r\n" {
		t.Errorf("TTL not found response = %q, want :-2\\r\\n", output)
	}
}

func TestCommandHandler_TTL_Success(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TTL"), []byte("tmss-test-session-id")}
	h.handleTTL(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Should return positive integer (remaining seconds)
	if !strings.HasPrefix(output, ":") {
		t.Errorf("TTL response should be integer, got %q", output)
	}
	if output == ":-2\r\n" || output == ":-1\r\n" {
		t.Errorf("TTL should return positive value for existing session, got %q", output)
	}
}

func TestCommandHandler_TTL_NoExpiration(t *testing.T) {
	sessionRepo := newMockSessionRepo()
	tokenRepo := newMockTokenRepo()
	apiKeyRepo := newMockAPIKeyRepo()

	tokenSvc := service.NewTokenService(tokenRepo, nil)
	sessionSvc := service.NewSessionService(sessionRepo, tokenSvc)
	authSvc := service.NewAuthService(apiKeyRepo, nil)

	srv := New(nil, sessionSvc, tokenSvc, authSvc, nil)
	h := srv.handler

	// Add a session with no expiration (ExpiresAt == 0)
	sessionRepo.sessions["tmss-no-expire-session1234"] = &domain.Session{
		ID:         "tmss-no-expire-session1234",
		UserID:     "user123",
		TokenHash:  "hash123",
		ExpiresAt:  0, // No expiration
		CreatedAt:  time.Now().UnixMilli(),
		LastActive: time.Now().UnixMilli(),
	}

	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TTL"), []byte("tmss-no-expire-session1234")}
	h.handleTTL(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Should return -1 for no expiration
	if output != ":-1\r\n" {
		t.Errorf("TTL no-expire response = %q, want :-1\\r\\n", output)
	}
}

// ============================================================
// Test: EXISTS command
// ============================================================

func TestCommandHandler_Exists_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXISTS")}
	h.handleExists(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Exists_NotFound(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXISTS"), []byte("non-existent")}
	h.handleExists(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":0\r\n" {
		t.Errorf("EXISTS not found response = %q, want :0\\r\\n", output)
	}
}

func TestCommandHandler_Exists_Found(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXISTS"), []byte("tmss-test-session-id")}
	h.handleExists(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":1\r\n" {
		t.Errorf("EXISTS found response = %q, want :1\\r\\n", output)
	}
}

func TestCommandHandler_Exists_Multiple(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("EXISTS"), []byte("tmss-test-session-id"), []byte("non-existent"), []byte("tmss-test-session-id")}
	h.handleExists(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Count matching keys: 2 (same key counted twice)
	if output != ":2\r\n" {
		t.Errorf("EXISTS multiple response = %q, want :2\\r\\n", output)
	}
}

// ============================================================
// Test: SCAN command
// ============================================================

func TestCommandHandler_Scan_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SCAN")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_Scan_InvalidCursor(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SCAN"), []byte("invalid")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "invalid cursor") {
		t.Errorf("expected invalid cursor error, got %q", output)
	}
}

func TestCommandHandler_Scan_Empty(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SCAN"), []byte("0")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Should return array with cursor and empty array
	if !strings.HasPrefix(output, "*2\r\n") {
		t.Errorf("SCAN should return array, got %q", output)
	}
}

func TestCommandHandler_Scan_WithMatch(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SCAN"), []byte("0"), []byte("MATCH"), []byte("tmss-*")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, "*2\r\n") {
		t.Errorf("SCAN should return array, got %q", output)
	}
}

func TestCommandHandler_Scan_WithCount(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SCAN"), []byte("0"), []byte("COUNT"), []byte("100")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, "*2\r\n") {
		t.Errorf("SCAN should return array, got %q", output)
	}
}

func TestCommandHandler_Scan_InvalidCount(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("SCAN"), []byte("0"), []byte("COUNT"), []byte("invalid")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "not an integer") {
		t.Errorf("expected integer error, got %q", output)
	}
}

func TestCommandHandler_Scan_SyntaxError(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Missing value for MATCH
	args := [][]byte{[]byte("SCAN"), []byte("0"), []byte("MATCH")}
	h.handleScan(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "syntax error") {
		t.Errorf("expected syntax error, got %q", output)
	}
}

// ============================================================
// Test: TM.CREATE command
// ============================================================

func TestCommandHandler_TMCreate_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.CREATE"), []byte("key")}
	h.handleTMCreate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_TMCreate_InvalidJSON(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.CREATE"), []byte("tmss-session"), []byte("invalid-json")}
	h.handleTMCreate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "invalid JSON") {
		t.Errorf("expected invalid JSON error, got %q", output)
	}
}

func TestCommandHandler_TMCreate_Success(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Valid session ID format: tmss- + 26-char ULID = 31 chars
	sessionID := "tmss-01ARZ3NDEKTSV4RRFFQ69G5FAX"
	jsonValue := `{"user_id":"user123","device_id":"device1"}`
	args := [][]byte{[]byte("TM.CREATE"), []byte(sessionID), []byte(jsonValue)}
	h.handleTMCreate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// Should return JSON with token
	if !strings.HasPrefix(output, "$") {
		t.Errorf("TM.CREATE should return bulk string, got %q", output)
	}
}

func TestCommandHandler_TMCreate_WithTTL(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	// Valid session ID format
	sessionID := "tmss-01ARZ3NDEKTSV4RRFFQ69G5FAY"
	jsonValue := `{"user_id":"user123"}`
	args := [][]byte{[]byte("TM.CREATE"), []byte(sessionID), []byte(jsonValue), []byte("TTL"), []byte("7200")}
	h.handleTMCreate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.HasPrefix(output, "$") {
		t.Errorf("TM.CREATE with TTL should return bulk string, got %q", output)
	}
}

func TestCommandHandler_TMCreate_InvalidTTL(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	jsonValue := `{"user_id":"user123"}`
	args := [][]byte{[]byte("TM.CREATE"), []byte("tmss-session"), []byte(jsonValue), []byte("TTL"), []byte("invalid")}
	h.handleTMCreate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "not an integer") {
		t.Errorf("expected integer error, got %q", output)
	}
}

// ============================================================
// Test: TM.VALIDATE command
// ============================================================

func TestCommandHandler_TMValidate_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.VALIDATE")}
	h.handleTMValidate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_TMValidate_InvalidToken(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.VALIDATE"), []byte("invalid-token")}
	h.handleTMValidate(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	// TM.VALIDATE returns error for invalid token
	if !strings.Contains(output, "TM-TOKN-4010") {
		t.Errorf("TM.VALIDATE invalid token response = %q, want error with TM-TOKN-4010", output)
	}
}

// ============================================================
// Test: TM.REVOKE_USER command
// ============================================================

func TestCommandHandler_TMRevokeUser_WrongArgs(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.REVOKE_USER")}
	h.handleTMRevokeUser(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if !strings.Contains(output, "wrong number of arguments") {
		t.Errorf("expected wrong arguments error, got %q", output)
	}
}

func TestCommandHandler_TMRevokeUser_NoSessions(t *testing.T) {
	h, _ := newTestCommandHandler()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.REVOKE_USER"), []byte("non-existent-user")}
	h.handleTMRevokeUser(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":0\r\n" {
		t.Errorf("TM.REVOKE_USER no sessions response = %q, want :0\\r\\n", output)
	}
}

func TestCommandHandler_TMRevokeUser_Success(t *testing.T) {
	h, _ := newTestCommandHandlerWithSession()
	tc := newTestConn()
	defer tc.Close()

	args := [][]byte{[]byte("TM.REVOKE_USER"), []byte("user123")}
	h.handleTMRevokeUser(tc.Conn, args)

	output := tc.FlushAndGetOutput()
	if output != ":1\r\n" {
		t.Errorf("TM.REVOKE_USER success response = %q, want :1\\r\\n", output)
	}
}

// ============================================================
// Test: sessionToRedisResponse
// ============================================================

func TestSessionToRedisResponse(t *testing.T) {
	session := &domain.Session{
		ID:           "tmss-test-id",
		UserID:       "user123",
		DeviceID:     "device1",
		TokenHash:    "hash123",
		IPAddress:    "192.168.1.1",
		UserAgent:    "TestAgent/1.0",
		CreatedAt:    time.Now().UnixMilli(),
		LastActive:   time.Now().UnixMilli(),
		ExpiresAt:    time.Now().Add(time.Hour).UnixMilli(),
		LastAccessIP: "192.168.1.2",
		Version:      1,
		Data:         map[string]string{"key": "value"},
	}

	resp := sessionToRedisResponse(session)

	if resp.ID != session.ID {
		t.Errorf("id = %q, want %q", resp.ID, session.ID)
	}
	if resp.UserID != session.UserID {
		t.Errorf("user_id = %q, want %q", resp.UserID, session.UserID)
	}
	if resp.DeviceID != session.DeviceID {
		t.Errorf("device_id = %q, want %q", resp.DeviceID, session.DeviceID)
	}

	// Test JSON marshaling
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}
	if !strings.Contains(string(data), "user123") {
		t.Errorf("JSON should contain user_id, got %s", data)
	}
}

// ============================================================
// Test Helper: Create CommandHandler with mock services
// Uses mock repos from redisserver_test.go
// ============================================================

func newTestCommandHandler() (*CommandHandler, *service.AuthService) {
	sessionRepo := newMockSessionRepo()
	tokenRepo := newMockTokenRepo()
	apiKeyRepo := newMockAPIKeyRepo()

	tokenSvc := service.NewTokenService(tokenRepo, nil)
	sessionSvc := service.NewSessionService(sessionRepo, tokenSvc)
	authSvc := service.NewAuthService(apiKeyRepo, nil)

	h := NewCommandHandler(sessionSvc, tokenSvc, authSvc, nil, nil)
	return h, authSvc
}

// newTestCommandHandlerWithSession creates a handler with a pre-existing session
func newTestCommandHandlerWithSession() (*CommandHandler, *service.AuthService) {
	sessionRepo := newMockSessionRepo()
	tokenRepo := newMockTokenRepo()
	apiKeyRepo := newMockAPIKeyRepo()

	// Create a test session
	ctx := context.Background()
	session := &domain.Session{
		ID:         "tmss-test-session-id",
		UserID:     "user123",
		DeviceID:   "device1",
		TokenHash:  "test-token-hash",
		CreatedAt:  time.Now().UnixMilli(),
		LastActive: time.Now().UnixMilli(),
		ExpiresAt:  time.Now().Add(time.Hour).UnixMilli(),
		Version:    1,
	}
	sessionRepo.Create(ctx, session)
	tokenRepo.sessions[session.TokenHash] = session

	tokenSvc := service.NewTokenService(tokenRepo, nil)
	sessionSvc := service.NewSessionService(sessionRepo, tokenSvc)
	authSvc := service.NewAuthService(apiKeyRepo, nil)

	h := NewCommandHandler(sessionSvc, tokenSvc, authSvc, nil, nil)
	return h, authSvc
}
