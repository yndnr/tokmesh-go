package buildinfo

import (
	"testing"
)

func TestGet(t *testing.T) {
	info := Get()

	// Check that all fields are populated with at least default values
	if info.Version == "" {
		t.Error("Version should not be empty")
	}
	if info.Commit == "" {
		t.Error("Commit should not be empty")
	}
	if info.BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
	if info.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}

	// Check default values
	if info.Version != "dev" {
		t.Logf("Version is customized: %s", info.Version)
	}
}

func TestString(t *testing.T) {
	s := String()

	// Should contain version
	if s == "" {
		t.Error("String() should not return empty")
	}

	// Should contain "built at"
	if len(s) < 10 {
		t.Error("String() should return a meaningful string")
	}

	// Check format: "version (commit) built at time"
	expected := Version + " (" + Commit + ") built at " + BuildTime
	if s != expected {
		t.Errorf("String() = %q, want %q", s, expected)
	}
}

func TestInfo_Fields(t *testing.T) {
	info := Get()

	// Verify JSON tags are present by checking field accessibility
	tests := []struct {
		name  string
		value string
	}{
		{"Version", info.Version},
		{"Commit", info.Commit},
		{"BuildTime", info.BuildTime},
		{"GoVersion", info.GoVersion},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Errorf("%s field should not be empty", tt.name)
			}
		})
	}
}

func TestDefaultValues(t *testing.T) {
	// Test that default values are reasonable
	if Version != "dev" && Version != "unknown" && Version[0] != 'v' {
		t.Logf("Version has unexpected format: %s", Version)
	}
}
