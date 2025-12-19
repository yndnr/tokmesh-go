// Package domain defines the core domain models for TokMesh.
package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestDomainError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *DomainError
		expected string
	}{
		{
			name:     "error without details",
			err:      NewDomainError("TM-TEST-1000", "test message"),
			expected: "[TM-TEST-1000] test message",
		},
		{
			name:     "error with details",
			err:      NewDomainError("TM-TEST-1001", "test message").WithDetails("extra info"),
			expected: "[TM-TEST-1001] test message: extra info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDomainError_Is(t *testing.T) {
	err1 := NewDomainError("TM-TEST-1000", "message 1")
	err2 := NewDomainError("TM-TEST-1000", "message 2") // Same code, different message
	err3 := NewDomainError("TM-TEST-1001", "message 1") // Different code

	// Same code should match
	if !errors.Is(err1, err2) {
		t.Error("errors.Is should return true for same error code")
	}

	// Different code should not match
	if errors.Is(err1, err3) {
		t.Error("errors.Is should return false for different error code")
	}

	// Should not match non-DomainError
	if errors.Is(err1, fmt.Errorf("some error")) {
		t.Error("errors.Is should return false for non-DomainError")
	}
}

func TestDomainError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("underlying cause")
	err := NewDomainError("TM-TEST-1000", "wrapper").WithCause(cause)

	unwrapped := errors.Unwrap(err)
	if unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}

	// Without cause
	errNoCause := NewDomainError("TM-TEST-1000", "no cause")
	if errors.Unwrap(errNoCause) != nil {
		t.Error("Unwrap() should return nil when no cause")
	}
}

func TestDomainError_WithDetails(t *testing.T) {
	original := NewDomainError("TM-TEST-1000", "original message")
	withDetails := original.WithDetails("additional details")

	// Check original is unchanged
	if original.Details != "" {
		t.Error("WithDetails should not modify original error")
	}

	// Check new error has details
	if withDetails.Details != "additional details" {
		t.Errorf("Details = %q, want %q", withDetails.Details, "additional details")
	}

	// Check code and message are preserved
	if withDetails.Code != original.Code {
		t.Errorf("Code = %q, want %q", withDetails.Code, original.Code)
	}
	if withDetails.Message != original.Message {
		t.Errorf("Message = %q, want %q", withDetails.Message, original.Message)
	}
}

func TestDomainError_WithCause(t *testing.T) {
	original := NewDomainError("TM-TEST-1000", "original message")
	cause := fmt.Errorf("root cause")
	withCause := original.WithCause(cause)

	// Check original is unchanged
	if original.Cause != nil {
		t.Error("WithCause should not modify original error")
	}

	// Check new error has cause
	if withCause.Cause != cause {
		t.Errorf("Cause = %v, want %v", withCause.Cause, cause)
	}

	// Check code and message are preserved
	if withCause.Code != original.Code {
		t.Errorf("Code = %q, want %q", withCause.Code, original.Code)
	}
}

func TestDomainError_Wrap(t *testing.T) {
	original := NewDomainError("TM-TEST-1000", "original")
	cause := fmt.Errorf("cause")
	wrapped := original.Wrap(cause)

	if wrapped.Cause != cause {
		t.Errorf("Wrap() should set cause, got %v", wrapped.Cause)
	}
}

func TestIsDomainError(t *testing.T) {
	err := ErrSessionNotFound

	if !IsDomainError(err, "TM-SESS-4040") {
		t.Error("IsDomainError should return true for matching code")
	}

	if IsDomainError(err, "TM-SESS-9999") {
		t.Error("IsDomainError should return false for non-matching code")
	}

	if IsDomainError(fmt.Errorf("regular error"), "TM-SESS-4040") {
		t.Error("IsDomainError should return false for non-DomainError")
	}

	// Test with wrapped error
	wrapped := fmt.Errorf("wrapped: %w", ErrSessionNotFound)
	if !IsDomainError(wrapped, "TM-SESS-4040") {
		t.Error("IsDomainError should work with wrapped errors")
	}
}

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "domain error",
			err:      ErrSessionNotFound,
			expected: "TM-SESS-4040",
		},
		{
			name:     "wrapped domain error",
			err:      fmt.Errorf("wrapped: %w", ErrTokenMalformed),
			expected: "TM-TOKN-4000",
		},
		{
			name:     "regular error",
			err:      fmt.Errorf("regular error"),
			expected: "",
		},
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetErrorCode(tt.err); got != tt.expected {
				t.Errorf("GetErrorCode() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	// Verify all predefined errors have correct codes
	tests := []struct {
		err  *DomainError
		code string
	}{
		// Session errors
		{ErrSessionNotFound, "TM-SESS-4040"},
		{ErrSessionExpired, "TM-SESS-4041"},
		{ErrSessionConflict, "TM-SESS-4090"},
		{ErrSessionVersionConflict, "TM-SESS-4091"},
		{ErrSessionValidation, "TM-SESS-4001"},
		{ErrSessionQuotaExceeded, "TM-SESS-4002"},

		// Token errors
		{ErrTokenMalformed, "TM-TOKN-4000"},
		{ErrTokenInvalid, "TM-TOKN-4010"},
		{ErrTokenExpired, "TM-TOKN-4011"},
		{ErrTokenRevoked, "TM-TOKN-4012"},
		{ErrTokenHashConflict, "TM-TOKN-4090"},

		// Auth errors
		{ErrAPIKeyMissing, "TM-AUTH-4010"},
		{ErrAPIKeyInvalid, "TM-AUTH-4011"},
		{ErrAPIKeyDisabled, "TM-AUTH-4012"},
		{ErrTimestampSkew, "TM-AUTH-4014"},
		{ErrNonceReplay, "TM-AUTH-4015"},
		{ErrPermissionDenied, "TM-AUTH-4030"},
		{ErrIPNotAllowed, "TM-AUTH-4031"},

		// System errors
		{ErrInternalServer, "TM-SYS-5000"},
		{ErrStorageError, "TM-SYS-5001"},
		{ErrServiceUnavailable, "TM-SYS-5030"},
		{ErrBadRequest, "TM-SYS-4000"},
		{ErrRateLimited, "TM-SYS-4290"},

		// Argument errors
		{ErrInvalidArgument, "TM-ARG-1001"},
		{ErrMissingArgument, "TM-ARG-1002"},
		{ErrArgumentConflict, "TM-ARG-1003"},

		// Admin errors
		{ErrAdminPermissionDenied, "TM-ADMIN-4030"},
		{ErrAdminIPNotAllowed, "TM-ADMIN-4031"},
		{ErrAdminResourceNotFound, "TM-ADMIN-4041"},
		{ErrAdminOperationConflict, "TM-ADMIN-4091"},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Error code = %q, want %q", tt.err.Code, tt.code)
			}
			if tt.err.Message == "" {
				t.Error("Error message should not be empty")
			}
		})
	}
}

func TestErrorChaining(t *testing.T) {
	// Test chaining WithDetails and WithCause
	cause := fmt.Errorf("root cause")
	err := ErrSessionNotFound.
		WithDetails("session_id: tmss-xxx").
		WithCause(cause)

	// Verify all properties are preserved
	if err.Code != "TM-SESS-4040" {
		t.Errorf("Code = %q, want %q", err.Code, "TM-SESS-4040")
	}
	if err.Details != "session_id: tmss-xxx" {
		t.Errorf("Details = %q", err.Details)
	}
	if err.Cause != cause {
		t.Error("Cause should be preserved")
	}

	// Verify errors.Is still works
	if !errors.Is(err, ErrSessionNotFound) {
		t.Error("errors.Is should work after chaining")
	}
}
