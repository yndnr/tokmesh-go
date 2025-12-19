// Package output provides output formatting for tokmesh-cli.
package output

import (
	"fmt"
	"io"
	"time"
)

// Spinner displays a progress animation.
type Spinner struct {
	w       io.Writer
	message string
	frames  []string
	done    chan struct{}
}

// NewSpinner creates a new spinner.
func NewSpinner(w io.Writer, message string) *Spinner {
	return &Spinner{
		w:       w,
		message: message,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		done:    make(chan struct{}),
	}
}

// Start starts the spinner animation.
func (s *Spinner) Start() {
	go func() {
		i := 0
		for {
			select {
			case <-s.done:
				return
			default:
				fmt.Fprintf(s.w, "\r%s %s", s.frames[i%len(s.frames)], s.message)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	close(s.done)
	fmt.Fprintf(s.w, "\r\033[K") // Clear line
}

// Success stops the spinner with a success message.
func (s *Spinner) Success(message string) {
	close(s.done)
	fmt.Fprintf(s.w, "\r✓ %s\n", message)
}

// Fail stops the spinner with a failure message.
func (s *Spinner) Fail(message string) {
	close(s.done)
	fmt.Fprintf(s.w, "\r✗ %s\n", message)
}
