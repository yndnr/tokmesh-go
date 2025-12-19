// Package localserver provides the local management server.
package localserver

import "io"

// Handler handles local management commands.
type Handler struct{}

// NewHandler creates a new Handler.
func NewHandler() *Handler {
	return &Handler{}
}

// Execute executes a local management command.
func (h *Handler) Execute(w io.Writer, cmd string, args []string) error {
	switch cmd {
	case "status":
		return h.handleStatus(w)
	case "shutdown":
		return h.handleShutdown(w)
	case "reload":
		return h.handleReload(w)
	case "drain":
		return h.handleDrain(w)
	default:
		_, err := w.Write([]byte("unknown command: " + cmd + "\n"))
		return err
	}
}

func (h *Handler) handleStatus(w io.Writer) error {
	// TODO: Return server status (uptime, connections, memory, etc.)
	return nil
}

func (h *Handler) handleShutdown(w io.Writer) error {
	// TODO: Trigger graceful shutdown
	return nil
}

func (h *Handler) handleReload(w io.Writer) error {
	// TODO: Reload configuration (hot reload)
	return nil
}

func (h *Handler) handleDrain(w io.Writer) error {
	// TODO: Stop accepting new connections, drain existing
	return nil
}
