package ratelimit

import (
	"log"
	"net"
	"net/http"
	"strings"
)

// KeyExtractor derives the composite key and bare IP from an *http.Request
// (BLM §1.1, §2.1 step 2; U-B/U-H). It is a value object — pure function of
// the request + the TrustProxyHeaders flag; it holds no state.
//
// The composite key shape is LOCKED (BR-10): `ip|METHOD /path`. The IP is
// derived per BR-12: RemoteAddr host part by default; the leftmost non-empty
// trimmed X-Forwarded-For entry only when TrustProxyHeaders is true. Malformed
// input always falls back to a benign IP and NEVER panics (BR-13, SEC-02).
type KeyExtractor struct {
	// TrustProxyHeaders branches IP source (D2/ADR-009). Default false —
	// without a known proxy, XFF is spoofable and is ignored even if present.
	TrustProxyHeaders bool
}

// fallbackIP is the benign IP returned when IP derivation fails (BR-13).
// Loopback is the safest fallback: a bad-IP request is grouped with local
// traffic rather than rejected outright, consistent with fail-open spirit.
const fallbackIP = "127.0.0.1"

// Extract returns the composite key (`ip|METHOD /path`) and the bare IP.
// The route portion is exact (BR-10 — no normalization, no query string, no
// host): the middleware matches overrides against `r.Method + " " + r.URL.Path`
// verbatim, and the key mirrors that so a single lookup resolves both.
//
// Extract never panics. On any error (malformed RemoteAddr, malformed XFF,
// empty XFF when trusted) it logs a warning when XFF was the source and falls
// back to RemoteAddr or the loopback sentinel (BR-13/US7).
func (k KeyExtractor) Extract(r *http.Request) (compositeKey, ip string) {
	ip = k.deriveIP(r)
	route := r.Method + " " + r.URL.Path
	compositeKey = ip + "|" + route
	return compositeKey, ip
}

// deriveIP implements BR-12. The XFF branch is opt-in: when
// TrustProxyHeaders is false, XFF is IGNORED even if present (US1 scenario 5).
// When true, the leftmost non-empty trimmed entry wins (ADR-009 single-proxy
// convention). Malformed XFF falls back to RemoteAddr + a warning log
// (fail-safe, US7 scenario 3).
func (k KeyExtractor) deriveIP(r *http.Request) string {
	if k.TrustProxyHeaders {
		xff := r.Header.Get("X-Forwarded-For")
		if xff != "" {
			ip := parseXFFLeftmost(xff)
			if ip != "" {
				return ip
			}
			// XFF was present but unparseable — fail-safe fallback (BR-13).
			log.Printf("rate_limit: malformed X-Forwarded-For %q; falling back to RemoteAddr", xff)
		}
		// Empty XFF when trusted — fall through to RemoteAddr (BR-12).
	}
	return remoteAddrHost(r)
}

// parseXFFLeftmost returns the leftmost non-empty trimmed entry of an
// X-Forwarded-For header, or "" if none parse (BR-12). It does NOT validate
// that the entry is a real IP — the limiter keys on the string verbatim,
// and a spoofed-but-non-empty value is still a usable key. An empty result
// signals "fall back to RemoteAddr" to the caller.
func parseXFFLeftmost(xff string) string {
	// XFF is comma-separated; the leftmost is the originating client under
	// the single-proxy convention (ADR-009).
	for _, part := range strings.Split(xff, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		return entry
	}
	return ""
}

// remoteAddrHost returns the host portion of r.RemoteAddr, splitting on the
// last colon (BR-12). Malformed input returns the loopback fallback and
// never panics (BR-13, SEC-02). An empty RemoteAddr also falls back.
func remoteAddrHost(r *http.Request) string {
	if r == nil {
		return fallbackIP
	}
	addr := r.RemoteAddr
	if addr == "" {
		return fallbackIP
	}
	// net.SplitHostPort handles "host:port" for both IPv4 and IPv6 forms.
	if host, _, err := net.SplitHostPort(addr); err == nil && host != "" {
		return host
	}
	// No port present (e.g., a Unix-domain socket or a malformed addr) —
	// use the raw string if it looks like an IP, else the fallback.
	if addr != "" && net.ParseIP(addr) != nil {
		return addr
	}
	return fallbackIP
}