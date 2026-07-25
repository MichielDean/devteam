package api

import (
	"net/http"
	"os"
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
		if isLocalhost(r) || matchesAdminSecret(r) {
			next(w, r)
			return
		}
		writeError(w, http.StatusUnauthorized, "admin_access_required", "admin access required")
	}
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