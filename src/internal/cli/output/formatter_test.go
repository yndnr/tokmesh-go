package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format   Format
		wide     bool
		wantType string
	}{
		{FormatJSON, false, "*output.JSONFormatter"},
		{FormatYAML, false, "*output.YAMLFormatter"},
		{FormatTable, false, "*output.TableFormatter"},
		{FormatTable, true, "*output.TableFormatter"},
		{"unknown", false, "*output.TableFormatter"}, // default to table
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			f := NewFormatter(tt.format, tt.wide)
			if f == nil {
				t.Fatal("NewFormatter returned nil")
			}

			// Check formatter type is correct
			switch tt.format {
			case FormatJSON:
				if _, ok := f.(*JSONFormatter); !ok {
					t.Error("expected JSONFormatter")
				}
			case FormatYAML:
				if _, ok := f.(*YAMLFormatter); !ok {
					t.Error("expected YAMLFormatter")
				}
			default:
				tf, ok := f.(*TableFormatter)
				if !ok {
					t.Error("expected TableFormatter")
				}
				if tt.wide && !tf.Wide {
					t.Error("expected Wide=true for table formatter")
				}
			}
		})
	}
}

func TestJSONFormatter_Format(t *testing.T) {
	f := &JSONFormatter{}

	t.Run("formats struct as JSON", func(t *testing.T) {
		data := struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}{
			Name:  "test",
			Value: 42,
		}

		var buf bytes.Buffer
		err := f.Format(&buf, data)
		if err != nil {
			t.Fatalf("Format() error = %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, `"name": "test"`) {
			t.Error("Format() missing name field")
		}
		if !strings.Contains(output, `"value": 42`) {
			t.Error("Format() missing value field")
		}
	})

	t.Run("formats slice as JSON", func(t *testing.T) {
		data := []string{"a", "b", "c"}

		var buf bytes.Buffer
		err := f.Format(&buf, data)
		if err != nil {
			t.Fatalf("Format() error = %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, `"a"`) {
			t.Error("Format() missing element a")
		}
	})

	t.Run("formats map as JSON", func(t *testing.T) {
		data := map[string]int{"key": 123}

		var buf bytes.Buffer
		err := f.Format(&buf, data)
		if err != nil {
			t.Fatalf("Format() error = %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, `"key": 123`) {
			t.Error("Format() missing key field")
		}
	})

	t.Run("formats nil as JSON", func(t *testing.T) {
		var buf bytes.Buffer
		err := f.Format(&buf, nil)
		if err != nil {
			t.Fatalf("Format(nil) error = %v", err)
		}

		output := strings.TrimSpace(buf.String())
		if output != "null" {
			t.Errorf("Format(nil) = %q, want 'null'", output)
		}
	})
}

func TestYAMLFormatter_Format(t *testing.T) {
	f := &YAMLFormatter{}

	t.Run("formats data (stub)", func(t *testing.T) {
		data := struct {
			Name string `json:"name"`
		}{
			Name: "test",
		}

		var buf bytes.Buffer
		err := f.Format(&buf, data)
		if err != nil {
			t.Fatalf("Format() error = %v", err)
		}

		// YAML formatter is a stub (TODO), so it returns empty
		// Just verify it doesn't error
	})
}
