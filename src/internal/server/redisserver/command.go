// Package redisserver provides a Redis protocol compatible server.
package redisserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yndnr/tokmesh-go/internal/core/domain"
	"github.com/yndnr/tokmesh-go/internal/core/service"
)

// formatRedisError converts an error to a Redis error string.
// For DomainErrors, returns "ERR <code> <message>".
// For other errors, returns "ERR <message>".
func formatRedisError(err error) string {
	var de *domain.DomainError
	if errors.As(err, &de) {
		return "ERR " + de.Code + " " + de.Message
	}
	return "ERR " + err.Error()
}

// rateLimiter implements a token bucket rate limiter per IP.
type rateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*tokenBucket
	rate    float64
}

type tokenBucket struct {
	tokens    float64
	lastCheck time.Time
}

func newRateLimiter(requestsPerSecond int) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    float64(requestsPerSecond),
	}
}

// allow checks if a request from the given IP should be allowed.
func (rl *rateLimiter) allow(ip string) bool {
	if rl.rate <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		b = &tokenBucket{
			tokens:    rl.rate,
			lastCheck: time.Now(),
		}
		rl.buckets[ip] = b
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > rl.rate {
		b.tokens = rl.rate
	}
	b.lastCheck = now

	// Check if we have tokens
	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// CommandHandler handles Redis commands.
type CommandHandler struct {
	sessionSvc  *service.SessionService
	tokenSvc    *service.TokenService
	authSvc     *service.AuthService
	logger      *slog.Logger
	rateLimiter *rateLimiter
}

// NewCommandHandler creates a new CommandHandler.
func NewCommandHandler(sessionSvc *service.SessionService, tokenSvc *service.TokenService, authSvc *service.AuthService, srv *Server, logger *slog.Logger) *CommandHandler {
	if logger == nil {
		logger = slog.Default()
	}

	var rl *rateLimiter
	if srv != nil && srv.cfg != nil && srv.cfg.RateLimit > 0 {
		rl = newRateLimiter(srv.cfg.RateLimit)
	}

	return &CommandHandler{
		sessionSvc:  sessionSvc,
		tokenSvc:    tokenSvc,
		authSvc:     authSvc,
		logger:      logger,
		rateLimiter: rl,
	}
}

// Handle handles a Redis command (RESP array of bulk strings).
func (h *CommandHandler) Handle(conn *Conn, args [][]byte) {
	if len(args) == 0 {
		_ = WriteError(conn.bw, "ERR no command")
		return
	}

	cmdName := normalizeCommandName(args[0])

	// Connection-level commands (do not require authentication).
	switch cmdName {
	case "PING":
		h.handlePing(conn, args)
		return
	case "AUTH":
		h.handleAuth(conn, args)
		return
	case "QUIT":
		h.handleQuit(conn, args)
		return
	}

	// All other commands require authentication.
	state := conn.GetState()
	if state == nil || !state.Authenticated {
		_ = WriteError(conn.bw, "NOAUTH Authentication required")
		return
	}

	// Rate limiting check (per-IP).
	if h.rateLimiter != nil {
		ip := conn.RemoteAddr().String()
		// Extract IP without port
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			ip = ip[:idx]
		}
		if !h.rateLimiter.allow(ip) {
			_ = WriteError(conn.bw, "ERR TM-RATE-4290 rate limit exceeded")
			return
		}
	}

	// Check permissions based on command.
	if !h.checkPermission(state, cmdName) {
		_ = WriteError(conn.bw, "ERR TM-AUTH-4030 permission denied for command '"+cmdName+"'")
		return
	}

	switch cmdName {
	case "GET":
		h.handleGet(conn, args)
	case "SET":
		h.handleSet(conn, args)
	case "DEL":
		h.handleDel(conn, args)
	case "EXPIRE":
		h.handleExpire(conn, args)
	case "TTL":
		h.handleTTL(conn, args)
	case "EXISTS":
		h.handleExists(conn, args)
	case "SCAN":
		h.handleScan(conn, args)
	case "TM.CREATE":
		h.handleTMCreate(conn, args)
	case "TM.VALIDATE":
		h.handleTMValidate(conn, args)
	case "TM.REVOKE_USER":
		h.handleTMRevokeUser(conn, args)
	default:
		_ = WriteError(conn.bw, "ERR unknown command '"+cmdName+"'")
	}
}

func (h *CommandHandler) checkPermission(state *ConnState, cmdName string) bool {
	if state == nil || state.APIKey == nil {
		return false
	}

	role := state.APIKey.Role
	if role == "admin" {
		return true
	}

	switch cmdName {
	case "GET", "TTL", "EXISTS", "SCAN", "TM.VALIDATE":
		return role == "validator" || role == "issuer"
	case "SET", "DEL", "EXPIRE", "TM.CREATE", "TM.REVOKE_USER":
		return role == "issuer"
	default:
		return false
	}
}

func (h *CommandHandler) handlePing(conn *Conn, args [][]byte) {
	if len(args) > 1 {
		_ = WriteBulk(conn.bw, args[1])
		return
	}
	_ = WriteSimpleString(conn.bw, "PONG")
}

// handleAuth handles the AUTH command.
// Supports:
//   - AUTH <key_id> <key_secret>
//   - AUTH <key_id>:<key_secret>
func (h *CommandHandler) handleAuth(conn *Conn, args [][]byte) {
	var keyID, keySecret string

	switch len(args) {
	case 2:
		parts := strings.SplitN(string(args[1]), ":", 2)
		if len(parts) != 2 {
			_ = WriteError(conn.bw, "ERR invalid AUTH format, expected 'key_id:key_secret' or 'key_id key_secret'")
			return
		}
		keyID, keySecret = parts[0], parts[1]
	case 3:
		keyID, keySecret = string(args[1]), string(args[2])
	default:
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'AUTH' command")
		return
	}

	ctx := context.Background()
	resp, err := h.authSvc.ValidateAPIKey(ctx, &service.ValidateAPIKeyRequest{
		KeyID:     keyID,
		KeySecret: keySecret,
	})
	if err != nil || !resp.Valid {
		_ = WriteError(conn.bw, "ERR TM-AUTH-4010 invalid credentials")
		return
	}

	conn.SetState(ConnState{
		Authenticated: true,
		APIKey: &service.APIKeyInfo{
			KeyID:   resp.APIKey.KeyID,
			Role:    string(resp.APIKey.Role),
			Name:    resp.APIKey.Name,
			Enabled: resp.APIKey.IsActive(),
		},
	})

	_ = WriteSimpleString(conn.bw, "OK")
}

func (h *CommandHandler) handleQuit(conn *Conn, _ [][]byte) {
	_ = WriteSimpleString(conn.bw, "OK")
	_ = conn.bw.Flush()
	_ = conn.Close()
}

// GET <key>
func (h *CommandHandler) handleGet(conn *Conn, args [][]byte) {
	if len(args) != 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'GET' command")
		return
	}

	sessionID := string(args[1])
	ctx := context.Background()
	session, err := h.sessionSvc.Get(ctx, &service.GetSessionRequest{SessionID: sessionID})
	if err != nil {
		if domain.IsDomainError(err, "TM-SESS-4040") || domain.IsDomainError(err, "TM-SESS-4041") {
			_ = WriteNullBulk(conn.bw)
			return
		}
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}

	data, err := json.Marshal(sessionToRedisResponse(session))
	if err != nil {
		_ = WriteError(conn.bw, "ERR failed to marshal session")
		return
	}
	_ = WriteBulk(conn.bw, data)
}

// SET <key> <value> [EX seconds]
//
// Creates or updates a session. The value must be a JSON object.
//
// Token handling policy:
//   - CREATE (key does not exist): token field is REQUIRED
//   - UPDATE (key exists): token field is IGNORED for security
//     Token rotation is not supported via SET. Use delete + create instead.
func (h *CommandHandler) handleSet(conn *Conn, args [][]byte) {
	if len(args) < 3 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'SET' command")
		return
	}

	sessionID := string(args[1])
	jsonValue := string(args[2])

	var ttl time.Duration
	for i := 3; i < len(args); i += 2 {
		if i+1 >= len(args) {
			_ = WriteError(conn.bw, "ERR syntax error")
			return
		}
		opt := strings.ToUpper(string(args[i]))
		if opt == "EX" {
			seconds, err := strconv.ParseInt(string(args[i+1]), 10, 64)
			if err != nil {
				_ = WriteError(conn.bw, "ERR value is not an integer or out of range")
				return
			}
			ttl = time.Duration(seconds) * time.Second
		}
	}

	var reqData sessionSetRequest
	if err := json.Unmarshal([]byte(jsonValue), &reqData); err != nil {
		_ = WriteError(conn.bw, "ERR invalid JSON value")
		return
	}

	ctx := context.Background()

	existing, err := h.sessionSvc.Get(ctx, &service.GetSessionRequest{SessionID: sessionID})
	if err != nil && !domain.IsDomainError(err, "TM-SESS-4040") {
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}

	if existing == nil {
		if reqData.Token == "" {
			_ = WriteError(conn.bw, "ERR TM-ARG-4001 token is required when creating new session with SET")
			return
		}
		if ttl == 0 {
			ttl = 24 * time.Hour
		}

		_, err := h.sessionSvc.CreateWithToken(ctx, &service.CreateSessionWithTokenRequest{
			SessionID: sessionID,
			UserID:    reqData.UserID,
			Token:     reqData.Token,
			DeviceID:  reqData.DeviceID,
			Data:      reqData.Data,
			TTL:       ttl,
		})
		if err != nil {
			_ = WriteError(conn.bw, formatRedisError(err))
			return
		}
		} else {
			// Update existing session.
			// SECURITY: Token rotation via SET is explicitly NOT supported.
			// Token changes should happen by deleting and recreating the session.
			if reqData.Token != "" {
				_ = WriteError(conn.bw, "ERR TM-ARG-4003 token rotation via SET not supported, recreate session instead")
				return
			}

		updateReq := &service.UpdateSessionRequest{SessionID: sessionID}
		if reqData.UserID != "" {
			updateReq.UserID = reqData.UserID
		}
		if reqData.DeviceID != "" {
			updateReq.DeviceID = reqData.DeviceID
		}
		if reqData.Data != nil {
			updateReq.Data = reqData.Data
		}
		if ttl > 0 {
			updateReq.TTL = ttl
		}
		_, err := h.sessionSvc.Update(ctx, updateReq)
		if err != nil {
			_ = WriteError(conn.bw, formatRedisError(err))
			return
		}
	}

	_ = WriteSimpleString(conn.bw, "OK")
}

// DEL <key> ...
func (h *CommandHandler) handleDel(conn *Conn, args [][]byte) {
	if len(args) < 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'DEL' command")
		return
	}
	if len(args) > 1001 {
		_ = WriteError(conn.bw, "ERR TM-ARG-4002 maximum 1000 keys per DEL command")
		return
	}

	ctx := context.Background()
	deleted := 0
	for i := 1; i < len(args); i++ {
		sessionID := string(args[i])
		_, err := h.sessionSvc.Revoke(ctx, &service.RevokeSessionRequest{SessionID: sessionID})
		if err == nil {
			deleted++
		}
	}

	_ = WriteInteger(conn.bw, int64(deleted))
}

// EXPIRE <key> <seconds>
func (h *CommandHandler) handleExpire(conn *Conn, args [][]byte) {
	if len(args) != 3 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'EXPIRE' command")
		return
	}

	sessionID := string(args[1])
	seconds, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		_ = WriteError(conn.bw, "ERR value is not an integer or out of range")
		return
	}

	ctx := context.Background()
	_, err = h.sessionSvc.Renew(ctx, &service.RenewSessionRequest{
		SessionID: sessionID,
		TTL:       time.Duration(seconds) * time.Second,
	})
	if err != nil {
		if domain.IsDomainError(err, "TM-SESS-4040") {
			_ = WriteInteger(conn.bw, 0)
			return
		}
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}
	_ = WriteInteger(conn.bw, 1)
}

// TTL <key>
//
// Returns the remaining time to live of a session in seconds.
// Returns:
//   - -2 if the key does not exist
//   - -1 if the key exists but has no associated expire (no TTL set)
//   - Positive integer: remaining seconds until expiration
func (h *CommandHandler) handleTTL(conn *Conn, args [][]byte) {
	if len(args) != 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'TTL' command")
		return
	}

	sessionID := string(args[1])
	ctx := context.Background()
	session, err := h.sessionSvc.Get(ctx, &service.GetSessionRequest{SessionID: sessionID})
	if err != nil {
		if domain.IsDomainError(err, "TM-SESS-4040") {
			_ = WriteInteger(conn.bw, -2)
			return
		}
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}

	// ExpiresAt == 0 means no expiration set
	if session.ExpiresAt == 0 {
		_ = WriteInteger(conn.bw, -1)
		return
	}

	expiresAt := time.UnixMilli(session.ExpiresAt)
	remaining := time.Until(expiresAt)
	if remaining < 0 {
		_ = WriteInteger(conn.bw, -2)
		return
	}
	_ = WriteInteger(conn.bw, int64(remaining.Seconds()))
}

// EXISTS <key> [key ...]
func (h *CommandHandler) handleExists(conn *Conn, args [][]byte) {
	if len(args) < 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'EXISTS' command")
		return
	}

	ctx := context.Background()
	count := 0
	for i := 1; i < len(args); i++ {
		sessionID := string(args[i])
		_, err := h.sessionSvc.Get(ctx, &service.GetSessionRequest{SessionID: sessionID})
		if err == nil {
			count++
		}
	}
	_ = WriteInteger(conn.bw, int64(count))
}

// SCAN <cursor> [MATCH pattern] [COUNT count]
func (h *CommandHandler) handleScan(conn *Conn, args [][]byte) {
	if len(args) < 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'SCAN' command")
		return
	}

	cursor, err := strconv.ParseInt(string(args[1]), 10, 64)
	if err != nil {
		_ = WriteError(conn.bw, "ERR invalid cursor")
		return
	}

	var pattern string
	count := 10
	for i := 2; i < len(args); i += 2 {
		if i+1 >= len(args) {
			_ = WriteError(conn.bw, "ERR syntax error")
			return
		}
		opt := strings.ToUpper(string(args[i]))
		switch opt {
		case "MATCH":
			pattern = string(args[i+1])
		case "COUNT":
			c, err := strconv.Atoi(string(args[i+1]))
			if err != nil {
				_ = WriteError(conn.bw, "ERR value is not an integer or out of range")
				return
			}
			count = c
		}
	}

	ctx := context.Background()
	filter := &service.SessionFilter{
		Page:     int(cursor) + 1,
		PageSize: count,
	}
	resp, err := h.sessionSvc.List(ctx, &service.ListSessionsRequest{Filter: filter})
	if err != nil {
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}

	// Filter by MATCH pattern if specified
	var filteredItems []*domain.Session
	if pattern != "" && pattern != "*" {
		for _, item := range resp.Items {
			if matchGlob(pattern, item.ID) {
				filteredItems = append(filteredItems, item)
			}
		}
	} else {
		filteredItems = resp.Items
	}

	nextCursor := "0"
	if len(resp.Items) == count {
		nextCursor = strconv.FormatInt(cursor+1, 10)
	}

	_ = WriteArrayHeader(conn.bw, 2)
	_ = WriteBulkString(conn.bw, nextCursor)
	_ = WriteArrayHeader(conn.bw, len(filteredItems))
	for _, item := range filteredItems {
		_ = WriteBulkString(conn.bw, item.ID)
	}
}

// matchGlob matches a string against a simple glob pattern.
// Supports * as wildcard that matches any characters.
// Examples:
//   - "tmss-*" matches "tmss-abc123"
//   - "*-user1" matches "tmss-user1"
//   - "tmss-*-device" matches "tmss-abc-device"
func matchGlob(pattern, s string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == "" {
		return s == ""
	}

	// Simple case: no wildcards
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}

	// Single trailing wildcard (prefix match): "prefix*"
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(s, prefix)
	}

	// Single leading wildcard (suffix match): "*suffix"
	if strings.HasPrefix(pattern, "*") && !strings.Contains(pattern[1:], "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(s, suffix)
	}

	// General case: multiple wildcards
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}

	// First part must be a prefix (if not empty)
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]

	// Middle parts must appear in order
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(s, parts[i])
		if idx < 0 {
			return false
		}
		s = s[idx+len(parts[i]):]
	}

	// Last part must be a suffix (if not empty)
	if len(parts) > 1 && parts[len(parts)-1] != "" {
		return strings.HasSuffix(s, parts[len(parts)-1])
	}

	return true
}

// TM.CREATE <key> <value> [TTL seconds]
func (h *CommandHandler) handleTMCreate(conn *Conn, args [][]byte) {
	if len(args) < 3 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'TM.CREATE' command")
		return
	}

	sessionID := string(args[1])
	jsonValue := string(args[2])

	ttl := 24 * time.Hour
	if len(args) >= 5 && strings.ToUpper(string(args[3])) == "TTL" {
		seconds, err := strconv.ParseInt(string(args[4]), 10, 64)
		if err != nil {
			_ = WriteError(conn.bw, "ERR value is not an integer or out of range")
			return
		}
		ttl = time.Duration(seconds) * time.Second
	}

	var reqData sessionSetRequest
	if err := json.Unmarshal([]byte(jsonValue), &reqData); err != nil {
		_ = WriteError(conn.bw, "ERR invalid JSON value")
		return
	}

	ctx := context.Background()
	resp, err := h.sessionSvc.CreateWithID(ctx, &service.CreateSessionWithIDRequest{
		SessionID: sessionID,
		UserID:    reqData.UserID,
		DeviceID:  reqData.DeviceID,
		Data:      reqData.Data,
		TTL:       ttl,
	})
	if err != nil {
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}

	result := map[string]any{
		"session_id": resp.SessionID,
		"token":      resp.Token,
		"expires_at": time.UnixMilli(resp.ExpiresAt).Format(time.RFC3339),
	}
	data, err := json.Marshal(result)
	if err != nil {
		_ = WriteError(conn.bw, "ERR failed to marshal response")
		return
	}
	_ = WriteBulk(conn.bw, data)
}

// TM.VALIDATE <token>
func (h *CommandHandler) handleTMValidate(conn *Conn, args [][]byte) {
	if len(args) != 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'TM.VALIDATE' command")
		return
	}

	token := string(args[1])
	ctx := context.Background()

	resp, err := h.tokenSvc.Validate(ctx, &service.ValidateTokenRequest{Token: token})
	if err != nil || !resp.Valid {
		_ = WriteError(conn.bw, "ERR TM-TOKN-4010 Token invalid")
		return
	}

	_ = WriteSimpleString(conn.bw, "OK")
}

// TM.REVOKE_USER <user_id>
func (h *CommandHandler) handleTMRevokeUser(conn *Conn, args [][]byte) {
	if len(args) != 2 {
		_ = WriteError(conn.bw, "ERR wrong number of arguments for 'TM.REVOKE_USER' command")
		return
	}

	userID := string(args[1])
	ctx := context.Background()

	resp, err := h.sessionSvc.RevokeByUser(ctx, &service.RevokeByUserRequest{UserID: userID})
	if err != nil {
		_ = WriteError(conn.bw, formatRedisError(err))
		return
	}
	_ = WriteInteger(conn.bw, int64(resp.RevokedCount))
}

// sessionSetRequest represents the JSON structure for SET/TM.CREATE commands.
type sessionSetRequest struct {
	UserID   string            `json:"user_id,omitempty"`
	Token    string            `json:"token,omitempty"`
	DeviceID string            `json:"device_id,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

// sessionRedisResponse represents the JSON structure for GET command responses.
type sessionRedisResponse struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	DeviceID     string            `json:"device_id,omitempty"`
	IPAddress    string            `json:"ip_address,omitempty"`
	UserAgent    string            `json:"user_agent,omitempty"`
	CreatedAt    string            `json:"created_at"`
	ExpiresAt    string            `json:"expires_at"`
	LastActive   string            `json:"last_active"`
	LastAccessIP string            `json:"last_access_ip,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
}

func sessionToRedisResponse(s *domain.Session) *sessionRedisResponse {
	return &sessionRedisResponse{
		ID:           s.ID,
		UserID:       s.UserID,
		DeviceID:     s.DeviceID,
		IPAddress:    s.IPAddress,
		UserAgent:    s.UserAgent,
		CreatedAt:    time.UnixMilli(s.CreatedAt).Format(time.RFC3339),
		ExpiresAt:    time.UnixMilli(s.ExpiresAt).Format(time.RFC3339),
		LastActive:   time.UnixMilli(s.LastActive).Format(time.RFC3339),
		LastAccessIP: s.LastAccessIP,
		Data:         s.Data,
	}
}
