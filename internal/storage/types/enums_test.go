package types

import (
	"encoding/json"
	"testing"
)

func TestDeviceType_String(t *testing.T) {
	tests := []struct {
		dt   DeviceType
		want string
	}{
		{DeviceWeb, "web"},
		{DeviceMobile, "mobile"},
		{DeviceDesktop, "desktop"},
		{DeviceAPI, "api"},
		{DeviceIoT, "iot"},
		{DeviceType(99), "unknown(99)"},
	}

	for _, tt := range tests {
		if got := tt.dt.String(); got != tt.want {
			t.Errorf("DeviceType(%d).String() = %s, want %s", tt.dt, got, tt.want)
		}
	}
}

func TestParseDeviceType(t *testing.T) {
	tests := []struct {
		input   string
		want    DeviceType
		wantErr bool
	}{
		{"web", DeviceWeb, false},
		{"mobile", DeviceMobile, false},
		{"desktop", DeviceDesktop, false},
		{"api", DeviceAPI, false},
		{"iot", DeviceIoT, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseDeviceType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseDeviceType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseDeviceType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDeviceType_JSON(t *testing.T) {
	dt := DeviceWeb
	data, err := json.Marshal(dt)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	want := `"web"`
	if string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}

	var got DeviceType
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if got != dt {
		t.Errorf("Unmarshal = %v, want %v", got, dt)
	}
}

func TestSessionType_String(t *testing.T) {
	tests := []struct {
		st   SessionType
		want string
	}{
		{SessionNormal, "normal"},
		{SessionVIP, "vip"},
		{SessionAdmin, "admin"},
	}

	for _, tt := range tests {
		if got := tt.st.String(); got != tt.want {
			t.Errorf("SessionType(%d).String() = %s, want %s", tt.st, got, tt.want)
		}
	}
}

func TestTokenType_String(t *testing.T) {
	tests := []struct {
		tt   TokenType
		want string
	}{
		{TokenAccess, "access"},
		{TokenRefresh, "refresh"},
		{TokenAdmin, "admin"},
	}

	for _, tc := range tests {
		if got := tc.tt.String(); got != tc.want {
			t.Errorf("TokenType(%d).String() = %s, want %s", tc.tt, got, tc.want)
		}
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		s    Status
		want string
	}{
		{StatusActive, "active"},
		{StatusExpired, "expired"},
		{StatusRevoked, "revoked"},
	}

	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Status(%d).String() = %s, want %s", tt.s, got, tt.want)
		}
	}
}
