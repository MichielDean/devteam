package api

import (
	"net/http"
	"os"
	"strings"
)

// adminGuard wraps a PUT /api/config/* handler. The request is allowed if:
//   - r.RemoteAddr is 127.0.0.1 / ::1 / localhost (localhost pass-through), OR
//   - header X-Admin-Secret matches env var ADMIN_SECRET (shared secret).
// Otherwise 401 with {"error": "admin access required"}.
//
// GET handlers are NOT guarded (matches existing /api/features unguarded;
// NFR-SEC-02 guards writes only). Full RBAC is deferred (X6).
// Traces U-API-04, NFR-SEC-02.
func adminGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if isLocalhost(r.RemoteAddr) || matchesAdminSecret(r) {
			next(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "admin_access_required", "admin access required")
	}
}

// isLocalhost returns true if the remote address is a loopback address.
func isLocalhost(remoteAddr string) bool {
	// Strip port if present.
	host := remoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 && !strings.HasPrefix(host, "[") {
		host = host[:idx]
	} else if strings.HasPrefix(host, "[") {
		// IPv6 bracket form [::1]:port
		if end := strings.Index(host, "]"); end > 0 {
			host = host[1:end]
		}
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost" || host == ""
}

// matchesAdminSecret returns true if the X-Admin-Secret header matches the
// ADMIN_SECRET env var. If ADMIN_SECRET is unset, this always returns false
// (no shared secret configured → only localhost access allowed).
func matchesAdminSecret(r *http.Request) bool {
	secret := os.Getenv("ADMIN_SECRET")
	if secret == "" {
		return false
	}
	provided := r.Header.Get("X-Admin-Secret")
	return provided != "" && provided == secret
}