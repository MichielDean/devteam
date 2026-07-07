package github

import (
	"errors"
	"fmt"
)

// AuthErrorCode is the typed enum for auth-family failures (ADR-12, BR-AUTH-04).
// Callers branch via errors.As(err, &authErr) then switch authErr.Code — no
// string parsing. The CLI's W-10 block formatter (cmd/devteam/auth.go) does
// exactly this (nfr-design-specs §5.1).
type AuthErrorCode string

const (
	// ErrCodeInstallationNotFound: 404 from token exchange — the App installation
	// was revoked (nfr-design-specs §4.3, ADR-09). PAT fallback NOT engaged.
	ErrCodeInstallationNotFound AuthErrorCode = "installation_not_found"
	// ErrCodeInstallationSuspended: 403 — org admin suspended the installation.
	ErrCodeInstallationSuspended AuthErrorCode = "installation_suspended"
	// ErrCodeKeyRejected: 401 from token exchange — bad JWT / bad App key.
	ErrCodeKeyRejected AuthErrorCode = "key_rejected"
	// ErrCodeKeyFileMissing: master key or App key file not found at the configured path.
	ErrCodeKeyFileMissing AuthErrorCode = "key_file_missing"
	// ErrCodeKeyFileInvalid: master key file not 32 bytes, or App key not valid PEM.
	ErrCodeKeyFileInvalid AuthErrorCode = "key_file_invalid"
	// ErrCodePATFallbackExhausted: primary auth failed (token-expiry-class) AND no
	// PAT is configured (nfr-design-specs §4.3).
	ErrCodePATFallbackExhausted AuthErrorCode = "pat_fallback_exhausted"
	// ErrCodeCiphertextTampered: GCM tag mismatch — ciphertext modified or master
	// key does not match (NFR-SEC-02 key-loss path).
	ErrCodeCiphertextTampered AuthErrorCode = "ciphertext_tampered"
	// ErrCodeGhCLINotFound: provider=gh AND gh not on PATH (NFR-PORT-03).
	ErrCodeGhCLINotFound AuthErrorCode = "gh_cli_not_found"
)

// RunbookSection maps each code to a runbook section pointer (BR-AUTH-05).
// The CLI formats this into the W-10 block: "see docs/github-app-setup.md §N".
func (c AuthErrorCode) RunbookSection() string {
	switch c {
	case ErrCodeInstallationNotFound, ErrCodeInstallationSuspended:
		return "§10"
	case ErrCodeKeyRejected:
		return "§9"
	case ErrCodeKeyFileMissing, ErrCodeKeyFileInvalid:
		return "§3"
	case ErrCodePATFallbackExhausted:
		return "§8"
	case ErrCodeCiphertextTampered:
		return "§7.4"
	case ErrCodeGhCLINotFound:
		return "§11"
	default:
		return "§11"
	}
}

// AuthError is the typed error for auth-family failures (ADR-12). Callers use
// errors.As to extract the Code and format the runbook pointer. The Error()
// string includes the runbook reference so a bare log of the error still
// points the operator at the fix (NFR-OPS-01).
type AuthError struct {
	Code           AuthErrorCode
	Message        string
	RunbookSection string // e.g. "§10"; if empty, derived from Code
}

func (e *AuthError) Error() string {
	section := e.RunbookSection
	if section == "" {
		section = e.Code.RunbookSection()
	}
	return fmt.Sprintf("%s: %s (see docs/github-app-setup.md %s)", e.Code, e.Message, section)
}

// authErrorFromCode is a constructor used by NativeClient/GhCLIClient to build
// a typed error with a message, deriving the runbook section from the code.
func authErrorFromCode(code AuthErrorCode, message string) *AuthError {
	return &AuthError{
		Code:           code,
		Message:        message,
		RunbookSection: code.RunbookSection(),
	}
}

// IsAuthError reports whether err is an *AuthError and returns it if so.
func IsAuthError(err error) (*AuthError, bool) {
	var ae *AuthError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}