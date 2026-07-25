package github

import (
	"io"
	"regexp"
)

// redactTokenURLs scrubs token-bearing URLs and bare token prefixes from an
// error string before it crosses the package boundary (ADR-12, CR-03,
// FR-AUDIT-05, nfr-design-specs §2.2).
//
// Patterns scrubbed:
//   - https://<token>@github.com/... → https://[redacted]@github.com/...
//     (<token> matches [^/@]+ — covers PATs, oauth tokens, installation tokens)
//   - token=<value> query params → token=[redacted]
//   - bare ghp_..., gho_..., ghs_... PAT/token prefixes (defense in depth)
//
// Idempotent (nfr-design-specs §2.2): applying twice is identical to once.
//
// Error-chain preservation: the redacted message replaces the outermost error
// message; the underlying error chain (errors.Is/As) is preserved by wrapping
// with %w on the original error. The redacted string is the new message.
func redactTokenURLs(err error) error {
	if err == nil {
		return nil
	}
	// Build a new error with the redacted message but wrapping the original
	// to preserve errors.Is/As. We use fmt.Errorf with %w on a sentinel that
	// carries the redacted string.
	redacted := redactString(err.Error())
	return &redactedError{msg: redacted, cause: err}
}

// redactedError carries a redacted message while preserving the error chain
// for errors.Is/As (nfr-design-specs §2.2 — "wraps with fmt.Errorf %w to
// preserve unwrapping"). We use a custom type instead of fmt.Errorf because
// %w requires the arg to be an error, and we want to REPLACE the message
// while keeping the chain.
type redactedError struct {
	msg   string
	cause error
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.cause }

// redactString is the string-form redaction used by the audit-details writer
// and the log sanitizer (nfr-design-specs §2.4, BR-AUDIT-01/04).
func redactString(s string) string {
	s = tokenURLRe.ReplaceAllString(s, "https://[redacted]@github.com")
	s = tokenQueryRe.ReplaceAllString(s, "token=[redacted]")
	s = ghpPrefixRe.ReplaceAllString(s, "ghp_[redacted]")
	s = ghoPrefixRe.ReplaceAllString(s, "gho_[redacted]")
	s = ghsPrefixRe.ReplaceAllString(s, "ghs_[redacted]")
	return s
}

var (
	// https://<token>@github.com/... — <token> is [^/@]+ (non-slash, non-at).
	tokenURLRe = regexp.MustCompile(`https://[^/@]+@github\.com`)
	// token=<value> query parameter.
	tokenQueryRe = regexp.MustCompile(`token=[^&\s]+`)
	// Bare PAT prefixes (defense in depth — these appear in some error families).
	ghpPrefixRe = regexp.MustCompile(`ghp_[A-Za-z0-9]{20,}`)
	ghoPrefixRe = regexp.MustCompile(`gho_[A-Za-z0-9]{20,}`)
	ghsPrefixRe = regexp.MustCompile(`ghs_[A-Za-z0-9]{20,}`)
)

// redactWriter is an io.Writer wrapper that scrubs each Write call through
// redactString before forwarding (nfr-design-specs §2.3, Layer 2 log sanitizer).
// Wired in cmd/devteam/main.go startup after config load.
type redactWriter struct {
	w io.Writer
}

// NewRedactWriter wraps w so all writes are scrubbed of token-bearing strings.
func NewRedactWriter(w io.Writer) io.Writer {
	return &redactWriter{w: w}
}

func (rw *redactWriter) Write(p []byte) (int, error) {
	cleaned := redactString(string(p))
	_, err := rw.w.Write([]byte(cleaned))
	if err != nil {
		return 0, err
	}
	return len(p), nil // report the original length so callers don't loop
}