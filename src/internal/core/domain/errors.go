// Package domain defines the core domain models for TokMesh.
package domain

import (
	"errors"
	"fmt"
)

// DomainError represents a business domain error with a structured error code.
// Error codes follow the format defined in specs/governance/error-codes.md.
//
// @req RQ-0104
// @design DS-0104
type DomainError struct {
	Code    string // Error code (e.g., "TM-SESS-4040")
	Message string // Human-readable message
	Details string // Optional additional details
	Cause   error  // Underlying error (if any)
}

// Error implements the error interface.
func (e *DomainError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Unwrap() support.
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// Is implements errors.Is() support for error comparison.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// NewDomainError creates a new DomainError with the given code and message.
//
// @design DS-0104
func NewDomainError(code, message string) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
	}
}

// WithDetails returns a copy of the error with additional details.
func (e *DomainError) WithDetails(details string) *DomainError {
	return &DomainError{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Cause:   e.Cause,
	}
}

// WithCause returns a copy of the error wrapping the given cause.
func (e *DomainError) WithCause(cause error) *DomainError {
	return &DomainError{
		Code:    e.Code,
		Message: e.Message,
		Details: e.Details,
		Cause:   cause,
	}
}

// Wrap wraps an error with this domain error as the cause.
func (e *DomainError) Wrap(cause error) *DomainError {
	return e.WithCause(cause)
}

// IsDomainError checks if an error is a DomainError with the given code.
// If code is empty, it only checks if the error is a DomainError.
//
// @design DS-0104
func IsDomainError(err error, code string) bool {
	var de *DomainError
	if errors.As(err, &de) {
		if code == "" {
			return true // Only check if it's a DomainError
		}
		return de.Code == code
	}
	return false
}

// GetErrorCode extracts the error code from an error if it's a DomainError.
//
// @design DS-0104
func GetErrorCode(err error) string {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}

// ============================================================================
// Session Errors (SESS)
// Reference: specs/governance/error-codes.md Section 3.3
// ============================================================================

var (
	// ErrSessionNotFound indicates the requested session was not found.
	ErrSessionNotFound = NewDomainError("TM-SESS-4040", "session not found")

	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = NewDomainError("TM-SESS-4041", "session expired")

	// ErrSessionConflict indicates the session ID already exists.
	ErrSessionConflict = NewDomainError("TM-SESS-4090", "session id conflict")

	// ErrSessionVersionConflict indicates an optimistic lock conflict.
	ErrSessionVersionConflict = NewDomainError("TM-SESS-4091", "version conflict, please retry")

	// ErrSessionValidation indicates session data validation failed.
	ErrSessionValidation = NewDomainError("TM-SESS-4001", "session validation failed")

	// ErrSessionQuotaExceeded indicates user session quota exceeded.
	ErrSessionQuotaExceeded = NewDomainError("TM-SESS-4002", "user session quota exceeded")
)

// ============================================================================
// Token Errors (TOKN)
// Reference: specs/governance/error-codes.md Section 3.4
// ============================================================================

var (
	// ErrTokenMalformed indicates the token format is invalid.
	ErrTokenMalformed = NewDomainError("TM-TOKN-4000", "malformed token")

	// ErrTokenInvalid indicates the token is invalid (not found).
	ErrTokenInvalid = NewDomainError("TM-TOKN-4010", "invalid token")

	// ErrTokenExpired indicates the token has expired.
	ErrTokenExpired = NewDomainError("TM-TOKN-4011", "token expired")

	// ErrTokenRevoked indicates the token has been revoked.
	ErrTokenRevoked = NewDomainError("TM-TOKN-4012", "token revoked")

	// ErrTokenHashConflict indicates a token hash collision.
	ErrTokenHashConflict = NewDomainError("TM-TOKN-4090", "token hash conflict")
)

// ============================================================================
// Authentication Errors (AUTH)
// Reference: specs/governance/error-codes.md Section 3.2
// ============================================================================

var (
	// ErrAPIKeyMissing indicates no API key was provided.
	ErrAPIKeyMissing = NewDomainError("TM-AUTH-4010", "api key not provided")

	// ErrAPIKeyInvalid indicates the API key is invalid or does not exist.
	ErrAPIKeyInvalid = NewDomainError("TM-AUTH-4011", "invalid api key")

	// ErrAPIKeyDisabled indicates the API key has been disabled.
	ErrAPIKeyDisabled = NewDomainError("TM-AUTH-4012", "api key disabled")

	// ErrTimestampSkew indicates the request timestamp is out of acceptable window.
	ErrTimestampSkew = NewDomainError("TM-AUTH-4014", "timestamp out of acceptable window")

	// ErrNonceReplay indicates a nonce replay attack was detected.
	ErrNonceReplay = NewDomainError("TM-AUTH-4015", "nonce replay detected")

	// ErrPermissionDenied indicates insufficient permissions.
	ErrPermissionDenied = NewDomainError("TM-AUTH-4030", "permission denied")

	// ErrIPNotAllowed indicates the IP is not in the allowlist.
	ErrIPNotAllowed = NewDomainError("TM-AUTH-4031", "ip not in allowlist")

	// ErrAPIKeyValidation indicates API key validation failed.
	ErrAPIKeyValidation = NewDomainError("TM-AUTH-4001", "api key validation failed")

	// ErrAPIKeyNotFound indicates the API key was not found.
	ErrAPIKeyNotFound = NewDomainError("TM-AUTH-4040", "api key not found")

	// ErrAPIKeyConflict indicates the API key ID already exists.
	ErrAPIKeyConflict = NewDomainError("TM-AUTH-4090", "api key id conflict")
)

// ============================================================================
// System Errors (SYS)
// Reference: specs/governance/error-codes.md Section 3.1
// ============================================================================

var (
	// ErrInternalServer indicates an internal server error.
	ErrInternalServer = NewDomainError("TM-SYS-5000", "internal server error")

	// ErrStorageError indicates a storage layer error.
	ErrStorageError = NewDomainError("TM-SYS-5001", "storage error")

	// ErrServiceUnavailable indicates the service is temporarily unavailable.
	ErrServiceUnavailable = NewDomainError("TM-SYS-5030", "service unavailable")

	// ErrBadRequest indicates a malformed request.
	ErrBadRequest = NewDomainError("TM-SYS-4000", "bad request")

	// ErrRateLimited indicates too many requests.
	ErrRateLimited = NewDomainError("TM-SYS-4290", "too many requests")
)

// ============================================================================
// Argument Errors (ARG)
// Reference: specs/governance/error-codes.md Section 3.7
// ============================================================================

var (
	// ErrInvalidArgument indicates an invalid argument.
	ErrInvalidArgument = NewDomainError("TM-ARG-1001", "invalid argument")

	// ErrMissingArgument indicates a required argument is missing.
	ErrMissingArgument = NewDomainError("TM-ARG-1002", "missing required argument")

	// ErrArgumentConflict indicates conflicting arguments.
	ErrArgumentConflict = NewDomainError("TM-ARG-1003", "argument conflict")
)

// ============================================================================
// Admin Errors (ADMIN)
// Reference: specs/governance/error-codes.md Section 3.8
// ============================================================================

var (
	// ErrAdminPermissionDenied indicates admin role is required.
	ErrAdminPermissionDenied = NewDomainError("TM-ADMIN-4030", "admin role required")

	// ErrAdminIPNotAllowed indicates the admin IP is not in allowlist.
	ErrAdminIPNotAllowed = NewDomainError("TM-ADMIN-4031", "admin ip not allowed")

	// ErrAdminResourceNotFound indicates the admin resource was not found.
	ErrAdminResourceNotFound = NewDomainError("TM-ADMIN-4041", "admin resource not found")

	// ErrAdminOperationConflict indicates a conflicting admin operation.
	ErrAdminOperationConflict = NewDomainError("TM-ADMIN-4091", "admin operation conflict")
)
