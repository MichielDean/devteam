package api

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// AdminGuard is the write-endpoint guard for the admin/settings route group
// (ADR-D6, FR-ROUTE-03). It allows:
//
//   - localhost requests (RemoteAddr is 127.0.0.1, ::1, or localhost) — the
//     single-operator dev workflow requires zero config.
//   - requests carrying a matching X-Devteam-Admin-Token header — supports
//     the trusted-LAN-access case.
//
// All other requests get 401 with the structured error shape
// {"error":"UNAUTHORIZED","details":"..."}. The guard is fail-closed: when
// DEVTEAM_ADMIN_TOKEN is unset and the request is non-localhost, it rejects
// (no accidental open writes — FR-ROUTE-03).
//
// Read routes (GET) skip this middleware (FR-ROUTE-02) — the route
// registration in server.go only wraps write methods.
//
// The guard is an http.Handler middleware, not inline checks, so a future
// RBAC feature can swap it without touching route registration (FR-ROUTE-04,
// NFR-MAINT-03).
func AdminGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLocalhost(r) {
			next.ServeHTTP(w, r)
			return
		}
		token := os.Getenv("DEVTEAM_ADMIN_TOKEN")
		if token != "" && r.Header.Get("X-Devteam-Admin-Token") == token {
			next.ServeHTTP(w, r)
			return
		}
		// Fail-closed: no token configured (or wrong token) + non-localhost.
		writeJSON(w, http.StatusUnauthorized, ErrorResponse{
			Error:   "UNAUTHORIZED",
			Details: "Admin write endpoints require a localhost request or a matching X-Devteam-Admin-Token header.",
		})
	})
}

// isLocalhost reports whether the request originated from the loopback
// address. RemoteAddr is host:port; we strip the port before comparing.
// Both IPv4 (127.0.0.1) and IPv6 (::1) loopback are accepted.
func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// No port — compare the whole RemoteAddr.
		host = r.RemoteAddr
	}
	host = strings.TrimSpace(host)
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}