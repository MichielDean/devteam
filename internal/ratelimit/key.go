package ratelimit

import (
	"log"
	"net"
	"net/http"
	"strings"
)

// KeyExtractor derives the composite limiter key and the bare client IP from an
// *http.Request. The composite key is "ip|METHOD /path" (BR-10); the bare IP is
// derived per BR-12 (RemoteAddr host part by default; leftmost non-empty trimmed
// X-Forwarded-For entry when TrustProxyHeaders is true). Malformed IP sources
// fail safe to a benign fallback (loopback) and NEVER panic (BR-13).
//
// The route portion is exact — no normalization, no query string, no host — so
// it matches routing exactly.
type KeyExtractor struct {
	// TrustProxyHeaders enables X-Forwarded-For parsing. Defaults to false
	// (D2): without a known proxy, XFF is spoofable. The operator who sets this
	// true has confirmed their proxy overwrites XFF (BR-43).
	TrustProxyHeaders bool
}

// Key returns (compositeKey, ip) for the request. The composite key is
// ip + "|" + r.Method + " " + r.URL.Path. When TrustProxyHeaders is false (the
// default), the IP is the host part of r.RemoteAddr and XFF is IGNORED even if
// present (US1 scenario 5). When TrustProxyHeaders is true, the IP is the
// leftmost valid IP in X-Forwarded-For after trimming whitespace; if the
// leftmost entry is malformed or empty, the extractor falls back to RemoteAddr
// and logs a warning (US7 scenario 3) — it does NOT keep scanning rightward for
// a valid entry, because a malformed head of the XFF chain signals an
// untrustworthy proxy that may have appended its own client. Malformed
// RemoteAddr falls back to loopback "127.0.0.1" and never panics (BR-13).
func (k KeyExtractor) Key(r *http.Request) (key string, ip string) {
	ip = k.clientIP(r)
	route := r.Method + " " + r.URL.Path
	key = ip + "|" + route
	return key, ip
}

// clientIP returns the bare client IP for the request per BR-12/BR-13.
func (k KeyExtractor) clientIP(r *http.Request) string {
	if k.TrustProxyHeaders {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			// Leftmost valid IP after trimming. If the leftmost non-empty entry is
			// malformed, fall back to RemoteAddr + warn (do NOT scan rightward — a
			// malformed XFF head signals an untrustworthy proxy).
			parts := strings.Split(xff, ",")
			for _, p := range parts {
				cand := strings.TrimSpace(p)
				if cand == "" {
					continue
				}
				if isValidIP(cand) {
					return cand
				}
				// Malformed leftmost entry: fall back to RemoteAddr + warn.
				log.Printf("rate_limit: xff malformed entry %q, falling back to RemoteAddr", cand)
				return remoteAddrHost(r.RemoteAddr)
			}
			// All entries empty: fall back to RemoteAddr (no warning — empty isn't malformed).
			return remoteAddrHost(r.RemoteAddr)
		}
		// Empty XFF header: fall back to RemoteAddr (no warning per NDP-10).
		return remoteAddrHost(r.RemoteAddr)
	}
	// Default: RemoteAddr host part, XFF ignored even if present.
	return remoteAddrHost(r.RemoteAddr)
}

// remoteAddrHost extracts the host part of an host:port RemoteAddr string and
// fails safe to "127.0.0.1" on any malformed input (BR-13, never panics).
func remoteAddrHost(remoteAddr string) string {
	if remoteAddr == "" {
		return "127.0.0.1"
	}
	// SplitHostPort handles both "ip:port" and "[ipv6]:port". It returns an error
	// for bare hostnames (no port) or garbage; on error, fall back to the whole
	// string if it looks like a valid IP, else loopback.
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// Could be a bare IP (no port) or garbage.
		if isValidIP(remoteAddr) {
			return remoteAddr
		}
		return "127.0.0.1"
	}
	if host == "" {
		return "127.0.0.1"
	}
	return host
}

// isValidIP reports whether s is a parseable IP address (IPv4 or IPv6).
func isValidIP(s string) bool {
	return net.ParseIP(s) != nil
}