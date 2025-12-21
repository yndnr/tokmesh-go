package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestNewSpinner(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "Loading")

	if s == nil {
		t.Fatal("NewSpinner returned nil")
	}
	if s.w != &buf {
		t.Error("Spinner writer not set correctly")
	}
	if s.message != "Loading" {
		t.Errorf("Spinner message = %q, want 'Loading'", s.message)
	}
	if len(s.frames) == 0 {
		t.Error("Spinner frames should not be empty")
	}
	if s.done == nil {
		t.Error("Spinner done channel should be initialized")
	}
}

func TestSpinner_StartStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "Processing")

	// Start the spinner
	s.Start()

	// Let it run briefly
	time.Sleep(150 * time.Millisecond)

	// Stop the spinner
	s.Stop()

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	// Verify output contains carriage return (clearing)
	output := buf.String()
	if !strings.Contains(output, "\r") {
		t.Error("Spinner output should contain carriage return")
	}
}

func TestSpinner_Success(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "Loading")

	// Start the spinner
	s.Start()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Success should stop with checkmark
	s.Success("Done!")

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "✓") {
		t.Error("Success output should contain checkmark")
	}
	if !strings.Contains(output, "Done!") {
		t.Error("Success output should contain message")
	}
}

func TestSpinner_Fail(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "Loading")

	// Start the spinner
	s.Start()

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Fail should stop with X mark
	s.Fail("Error occurred")

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "✗") {
		t.Error("Fail output should contain X mark")
	}
	if !strings.Contains(output, "Error occurred") {
		t.Error("Fail output should contain error message")
	}
}

func TestSpinner_StopWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf, "Test")

	// Stop without starting - should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stop without Start caused panic: %v", r)
		}
	}()

	s.Stop()
}
