package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewProgressBar(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewProgressBar(buf, "Test")

	if bar == nil {
		t.Fatal("NewProgressBar returned nil")
	}
	if bar.title != "Test" {
		t.Errorf("title = %q, want %q", bar.title, "Test")
	}
	if bar.width != 40 {
		t.Errorf("width = %d, want %d", bar.width, 40)
	}
}

func TestProgressBar_SetTotal(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewProgressBar(buf, "Test")

	bar.SetTotal(1000)
	if bar.total != 1000 {
		t.Errorf("total = %d, want %d", bar.total, 1000)
	}
}

func TestProgressBar_Update(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewProgressBar(buf, "Download")

	bar.Update(50, 100)

	output := buf.String()
	if !strings.Contains(output, "Download") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "50%") {
		t.Error("output should contain percentage")
	}
}

func TestProgressBar_Increment(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewProgressBar(buf, "Test")

	bar.SetTotal(100)
	bar.Increment(25)
	bar.Increment(25)

	if bar.current != 50 {
		t.Errorf("current = %d, want %d", bar.current, 50)
	}
}

func TestProgressBar_Finish(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewProgressBar(buf, "Test")

	bar.SetTotal(100)
	bar.Update(100, 100)
	bar.Finish()

	output := buf.String()
	if !strings.Contains(output, "100%") {
		t.Error("output should contain 100%")
	}
}

func TestProgressBar_UnknownTotal(t *testing.T) {
	buf := &bytes.Buffer{}
	bar := NewProgressBar(buf, "Upload")

	// When total is 0 (unknown), should just show current bytes
	bar.Update(1024, 0)

	output := buf.String()
	if !strings.Contains(output, "Upload") {
		t.Error("output should contain title")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
