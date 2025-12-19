package redisserver

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Protocol limits to prevent DoS attacks.
const (
	// MaxArrayLen limits the number of elements in a RESP array.
	// Most Redis commands have <20 args; MSET could have more but we don't support it.
	MaxArrayLen = 1024

	// MaxBulkLen limits the size of a single bulk string (512KB).
	// Session JSON is typically <4KB; this provides ample headroom.
	MaxBulkLen = 512 * 1024

	// MaxInlineLen limits inline command line length (4KB).
	MaxInlineLen = 4 * 1024
)

var (
	ErrProtocol      = errors.New("resp: protocol error")
	ErrLimitExceeded = errors.New("resp: limit exceeded")
)

func ReadCommand(r *bufio.Reader) ([][]byte, error) {
	b, err := r.Peek(1)
	if err != nil {
		return nil, err
	}

	switch b[0] {
	case '*':
		return readArrayCommand(r)
	default:
		// Inline command (rare, but used by some clients): "PING\r\n"
		line, err := readLine(r, MaxInlineLen)
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return nil, nil
		}
		parts := strings.Fields(line)
		out := make([][]byte, 0, len(parts))
		for _, p := range parts {
			out = append(out, []byte(p))
		}
		return out, nil
	}
}

func readArrayCommand(r *bufio.Reader) ([][]byte, error) {
	// "*<n>\r\n"
	line, err := readLine(r, 64) // Array header is short: "*<number>\r\n"
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[0] != '*' {
		return nil, fmt.Errorf("%w: expected array", ErrProtocol)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid array length", ErrProtocol)
	}
	if n <= 0 {
		return nil, nil
	}
	if n > MaxArrayLen {
		return nil, fmt.Errorf("%w: array length %d exceeds limit %d", ErrLimitExceeded, n, MaxArrayLen)
	}

	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		arg, err := readBulkString(r)
		if err != nil {
			return nil, err
		}
		out = append(out, arg)
	}
	return out, nil
}

func readBulkString(r *bufio.Reader) ([]byte, error) {
	line, err := readLine(r, 64) // Bulk header is short: "$<number>\r\n"
	if err != nil {
		return nil, err
	}
	if len(line) < 2 || line[0] != '$' {
		// Support simple strings as args (best-effort).
		if len(line) >= 2 && line[0] == '+' {
			return []byte(line[1:]), nil
		}
		return nil, fmt.Errorf("%w: expected bulk string", ErrProtocol)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[1:]))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid bulk length", ErrProtocol)
	}
	if n == -1 {
		return nil, nil
	}
	if n < 0 {
		return nil, fmt.Errorf("%w: invalid bulk length", ErrProtocol)
	}
	if n > MaxBulkLen {
		return nil, fmt.Errorf("%w: bulk length %d exceeds limit %d", ErrLimitExceeded, n, MaxBulkLen)
	}

	buf := make([]byte, n+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	if !bytes.HasSuffix(buf, []byte("\r\n")) {
		return nil, fmt.Errorf("%w: invalid bulk terminator", ErrProtocol)
	}
	return buf[:len(buf)-2], nil
}

func readLine(r *bufio.Reader, maxLen int) (string, error) {
	if maxLen <= 0 {
		return "", fmt.Errorf("%w: invalid maxLen", ErrProtocol)
	}

	var buf []byte
	for {
		frag, err := r.ReadSlice('\n')
		if err == nil {
			buf = append(buf, frag...)
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			buf = append(buf, frag...)
			if len(buf) > maxLen {
				return "", fmt.Errorf("%w: line length exceeds limit %d", ErrLimitExceeded, maxLen)
			}
			continue
		}
		return "", err
	}

	if len(buf) > maxLen {
		return "", fmt.Errorf("%w: line length exceeds limit %d", ErrLimitExceeded, maxLen)
	}
	if len(buf) < 2 || !bytes.HasSuffix(buf, []byte("\r\n")) {
		return "", fmt.Errorf("%w: missing CRLF", ErrProtocol)
	}

	buf = bytes.TrimSuffix(buf, []byte("\r\n"))
	return string(buf), nil
}

func WriteSimpleString(w *bufio.Writer, s string) error {
	_, err := w.WriteString("+" + s + "\r\n")
	return err
}

func WriteError(w *bufio.Writer, s string) error {
	_, err := w.WriteString("-" + s + "\r\n")
	return err
}

func WriteInteger(w *bufio.Writer, n int64) error {
	_, err := w.WriteString(":" + strconv.FormatInt(n, 10) + "\r\n")
	return err
}

func WriteNullBulk(w *bufio.Writer) error {
	_, err := w.WriteString("$-1\r\n")
	return err
}

func WriteBulk(w *bufio.Writer, b []byte) error {
	if b == nil {
		return WriteNullBulk(w)
	}
	if _, err := w.WriteString("$" + strconv.Itoa(len(b)) + "\r\n"); err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

func WriteBulkString(w *bufio.Writer, s string) error {
	return WriteBulk(w, []byte(s))
}

func WriteArrayHeader(w *bufio.Writer, n int) error {
	_, err := w.WriteString("*" + strconv.Itoa(n) + "\r\n")
	return err
}

func normalizeCommandName(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	// Uppercase ASCII without allocating for already uppercased tokens.
	if bytes.ContainsAny(b, "abcdefghijklmnopqrstuvwxyz") {
		return strings.ToUpper(string(b))
	}
	return string(b)
}
