package redisserver

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// ============================================================
// ReadCommand Tests - Array Format
// ============================================================

func TestReadCommand_Array(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "simple PING command",
			input: "*1\r\n$4\r\nPING\r\n",
			want:  []string{"PING"},
		},
		{
			name:  "GET command",
			input: "*2\r\n$3\r\nGET\r\n$6\r\nmykey1\r\n",
			want:  []string{"GET", "mykey1"},
		},
		{
			name:  "SET command with value",
			input: "*3\r\n$3\r\nSET\r\n$5\r\nmykey\r\n$7\r\nmyvalue\r\n",
			want:  []string{"SET", "mykey", "myvalue"},
		},
		{
			name:  "AUTH with two args",
			input: "*3\r\n$4\r\nAUTH\r\n$6\r\nkeyid1\r\n$10\r\nkeysecret1\r\n",
			want:  []string{"AUTH", "keyid1", "keysecret1"},
		},
		{
			name:  "empty array",
			input: "*0\r\n",
			want:  nil,
		},
		{
			name:  "null array",
			input: "*-1\r\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			got, err := ReadCommand(r)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d", len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if string(got[i]) != want {
					t.Errorf("arg[%d] = %q, want %q", i, string(got[i]), want)
				}
			}
		})
	}
}

// ============================================================
// ReadCommand Tests - Inline Format
// ============================================================

func TestReadCommand_Inline(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple PING",
			input: "PING\r\n",
			want:  []string{"PING"},
		},
		{
			name:  "QUIT",
			input: "QUIT\r\n",
			want:  []string{"QUIT"},
		},
		{
			name:  "inline with args",
			input: "GET mykey\r\n",
			want:  []string{"GET", "mykey"},
		},
		{
			name:  "empty line",
			input: "\r\n",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   \r\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			got, err := ReadCommand(r)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("len = %d, want %d", len(got), len(tt.want))
				return
			}

			for i, want := range tt.want {
				if string(got[i]) != want {
					t.Errorf("arg[%d] = %q, want %q", i, string(got[i]), want)
				}
			}
		})
	}
}

// ============================================================
// Pipeline Tests
// ============================================================

func TestReadCommand_Pipeline(t *testing.T) {
	// Multiple commands in a single input (pipeline)
	input := "*1\r\n$4\r\nPING\r\n*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n*1\r\n$4\r\nQUIT\r\n"
	r := bufio.NewReader(strings.NewReader(input))

	// First command: PING
	cmd1, err := ReadCommand(r)
	if err != nil {
		t.Fatalf("cmd1 error: %v", err)
	}
	if len(cmd1) != 1 || string(cmd1[0]) != "PING" {
		t.Errorf("cmd1 = %v, want [PING]", cmd1)
	}

	// Second command: GET key
	cmd2, err := ReadCommand(r)
	if err != nil {
		t.Fatalf("cmd2 error: %v", err)
	}
	if len(cmd2) != 2 || string(cmd2[0]) != "GET" || string(cmd2[1]) != "key" {
		t.Errorf("cmd2 = %v, want [GET key]", cmd2)
	}

	// Third command: QUIT
	cmd3, err := ReadCommand(r)
	if err != nil {
		t.Fatalf("cmd3 error: %v", err)
	}
	if len(cmd3) != 1 || string(cmd3[0]) != "QUIT" {
		t.Errorf("cmd3 = %v, want [QUIT]", cmd3)
	}
}

// ============================================================
// Protocol Limit Tests
// ============================================================

func TestReadCommand_ArrayLenLimit(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("*1025\r\n"))
	_, err := ReadCommand(r)
	if err == nil {
		t.Fatalf("ReadCommand() error = nil, want error")
	}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("error = %v, want ErrLimitExceeded", err)
	}
}

func TestReadCommand_BulkLenLimit(t *testing.T) {
	// Exceeds MaxBulkLen; ReadCommand should error before reading the body.
	r := bufio.NewReader(strings.NewReader("*1\r\n$524289\r\n"))
	_, err := ReadCommand(r)
	if err == nil {
		t.Fatalf("ReadCommand() error = nil, want error")
	}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("error = %v, want ErrLimitExceeded", err)
	}
}

func TestReadCommand_InlineLenLimit(t *testing.T) {
	line := strings.Repeat("A", MaxInlineLen+1) + "\r\n"
	r := bufio.NewReader(strings.NewReader(line))
	_, err := ReadCommand(r)
	if err == nil {
		t.Fatalf("ReadCommand() error = nil, want error")
	}
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("error = %v, want ErrLimitExceeded", err)
	}
}

// ============================================================
// Protocol Error Tests
// ============================================================

func TestReadCommand_InvalidProtocol(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "array without CRLF",
			input: "*2\n$3\nGET\n$3\nkey\n",
		},
		{
			name:  "invalid array count",
			input: "*abc\r\n",
		},
		{
			name:  "invalid bulk length",
			input: "*1\r\n$xyz\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := bufio.NewReader(strings.NewReader(tt.input))
			_, err := ReadCommand(r)

			if err == nil {
				t.Error("expected protocol error")
			}
		})
	}
}

func TestReadCommand_NullBulkString(t *testing.T) {
	// Null bulk string ($-1)
	input := "*2\r\n$3\r\nGET\r\n$-1\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	got, err := ReadCommand(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	if string(got[0]) != "GET" {
		t.Errorf("arg[0] = %q, want GET", got[0])
	}

	if got[1] != nil {
		t.Errorf("arg[1] = %v, want nil", got[1])
	}
}

// ============================================================
// Response Writer Tests
// ============================================================

func TestWriteSimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	_ = WriteSimpleString(w, "OK")
	_ = w.Flush()

	if buf.String() != "+OK\r\n" {
		t.Errorf("got %q, want +OK\\r\\n", buf.String())
	}
}

func TestWriteError(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	_ = WriteError(w, "ERR unknown command")
	_ = w.Flush()

	if buf.String() != "-ERR unknown command\r\n" {
		t.Errorf("got %q, want -ERR unknown command\\r\\n", buf.String())
	}
}

func TestWriteInteger(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{0, ":0\r\n"},
		{1, ":1\r\n"},
		{-1, ":-1\r\n"},
		{-2, ":-2\r\n"},
		{3600, ":3600\r\n"},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)

		_ = WriteInteger(w, tt.n)
		_ = w.Flush()

		if buf.String() != tt.want {
			t.Errorf("WriteInteger(%d) = %q, want %q", tt.n, buf.String(), tt.want)
		}
	}
}

func TestWriteNullBulk(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	_ = WriteNullBulk(w)
	_ = w.Flush()

	if buf.String() != "$-1\r\n" {
		t.Errorf("got %q, want $-1\\r\\n", buf.String())
	}
}

func TestWriteBulk(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"normal", []byte("hello"), "$5\r\nhello\r\n"},
		{"empty", []byte(""), "$0\r\n\r\n"},
		{"nil", nil, "$-1\r\n"},
		{"binary", []byte{0x00, 0x01, 0x02}, "$3\r\n\x00\x01\x02\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := bufio.NewWriter(&buf)

			_ = WriteBulk(w, tt.input)
			_ = w.Flush()

			if buf.String() != tt.want {
				t.Errorf("got %q, want %q", buf.String(), tt.want)
			}
		})
	}
}

func TestWriteArrayHeader(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "*0\r\n"},
		{1, "*1\r\n"},
		{5, "*5\r\n"},
		{100, "*100\r\n"},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)

		_ = WriteArrayHeader(w, tt.n)
		_ = w.Flush()

		if buf.String() != tt.want {
			t.Errorf("WriteArrayHeader(%d) = %q, want %q", tt.n, buf.String(), tt.want)
		}
	}
}

// ============================================================
// matchGlob Tests
// ============================================================

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		s       string
		want    bool
	}{
		// Wildcard matches everything
		{"*", "anything", true},
		{"*", "", true},

		// Exact match
		{"hello", "hello", true},
		{"hello", "world", false},

		// Prefix match
		{"tmss-*", "tmss-abc123", true},
		{"tmss-*", "tmss-", true},
		{"tmss-*", "other-abc", false},

		// Suffix match
		{"*-user1", "tmss-user1", true},
		{"*-user1", "session-user1", true},
		{"*-user1", "user1-session", false},

		// Contains match
		{"*abc*", "xyzabcdef", true},
		{"*abc*", "abcdef", true},
		{"*abc*", "xyzabc", true},
		{"*abc*", "xyz", false},

		// Multiple wildcards
		{"*-*-*", "a-b-c", true},
		{"*-*-*", "abc-def-ghi", true},
		{"*-*-*", "a-b", false},

		// Empty patterns
		{"", "", true},
		{"", "nonempty", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.s, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.s)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.s, got, tt.want)
			}
		})
	}
}

// ============================================================
// normalizeCommandName Tests
// ============================================================

func TestNormalizeCommandName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"GET", "GET"},
		{"get", "GET"},
		{"Get", "GET"},
		{"ping", "PING"},
		{"TM.CREATE", "TM.CREATE"},
		{"tm.create", "TM.CREATE"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeCommandName([]byte(tt.input))
		if got != tt.want {
			t.Errorf("normalizeCommandName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ============================================================
// Additional readBulkString Tests
// ============================================================

func TestReadBulkString_NegativeLength(t *testing.T) {
	// Negative length other than -1 should error
	input := "*1\r\n$-2\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := ReadCommand(r)

	if err == nil {
		t.Error("expected error for negative bulk length")
	}
}

func TestReadBulkString_SimpleString(t *testing.T) {
	// Simple string as fallback in bulk position
	input := "*2\r\n$3\r\nGET\r\n+simple\r\n"
	r := bufio.NewReader(strings.NewReader(input))
	got, err := ReadCommand(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	if string(got[1]) != "simple" {
		t.Errorf("arg[1] = %q, want %q", got[1], "simple")
	}
}

func TestReadBulkString_MissingTerminator(t *testing.T) {
	// Missing \r\n terminator
	input := "*1\r\n$4\r\ntest"
	r := bufio.NewReader(strings.NewReader(input))
	_, err := ReadCommand(r)

	if err == nil {
		t.Error("expected error for missing terminator")
	}
}

// ============================================================
// Rate Limiter Tests
// ============================================================

func TestRateLimiter_Allow(t *testing.T) {
	rl := newRateLimiter(10) // 10 requests per second

	// First 10 requests should be allowed
	for i := 0; i < 10; i++ {
		if !rl.allow("192.168.1.1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 11th request should be rejected
	if rl.allow("192.168.1.1") {
		t.Error("11th request should be rejected")
	}

	// Different IP should be allowed
	if !rl.allow("192.168.1.2") {
		t.Error("different IP should be allowed")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := newRateLimiter(100) // 100 requests per second

	// Exhaust the bucket
	for i := 0; i < 100; i++ {
		rl.allow("192.168.1.1")
	}

	// Should be rejected
	if rl.allow("192.168.1.1") {
		t.Error("should be rejected after exhausting bucket")
	}

	// Wait a bit for tokens to refill
	time.Sleep(50 * time.Millisecond)

	// Should be allowed again after refill
	if !rl.allow("192.168.1.1") {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	rl := newRateLimiter(0) // Disabled

	// Should always allow when disabled
	for i := 0; i < 1000; i++ {
		if !rl.allow("192.168.1.1") {
			t.Errorf("request %d should be allowed when rate limiter is disabled", i+1)
		}
	}
}
