package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestGuard_LocalhostAllowed verifies a request from 127.0.0.1 is allowed
// through the guard with no token configured (S-ROUTE-03 AC2 — the
// single-operator dev workflow requires zero config).
func TestGuard_LocalhostAllowed(t *testing.T) {
	os.Unsetenv("DEVTEAM_ADMIN_TOKEN")
	called := false
 guarded := AdminGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	w := httptest.NewRecorder()
	guarded.ServeHTTP(w, req)

	if !called {
		t.Error("handler not called for localhost request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// TestGuard_IPv6LocalhostAllowed verifies ::1 is treated as localhost too.
func TestGuard_IPv6LocalhostAllowed(t *testing.T) {
	os.Unsetenv("DEVTEAM_ADMIN_TOKEN")
	called := false
	guarded := AdminGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	req.RemoteAddr = "[::1]:54321"
	w := httptest.NewRecorder()
	guarded.ServeHTTP(w, req)

	if !called {
		t.Error("handler not called for ::1 request")
	}
}

// TestGuard_ValidTokenAllowed verifies a non-localhost request with a
// matching X-Devteam-Admin-Token header is allowed (S-ROUTE-03 AC2 — the
// trusted-LAN-access case).
func TestGuard_ValidTokenAllowed(t *testing.T) {
	os.Setenv("DEVTEAM_ADMIN_TOKEN", "secret-token")
	t.Cleanup(func() { os.Unsetenv("DEVTEAM_ADMIN_TOKEN") })

	called := false
	guarded := AdminGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	req.RemoteAddr = "10.0.0.5:54321"
	req.Header.Set("X-Devteam-Admin-Token", "secret-token")
	w := httptest.NewRecorder()
	guarded.ServeHTTP(w, req)

	if !called {
		t.Error("handler not called for valid-token request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// TestGuard_MissingOrWrongToken_401 verifies a non-localhost request without
// a token (or with the wrong token) is rejected with 401 and produces no
// side effect (S-ROUTE-03 AC1, NFR-SEC-02).
func TestGuard_MissingOrWrongToken_401(t *testing.T) {
	os.Setenv("DEVTEAM_ADMIN_TOKEN", "secret-token")
	t.Cleanup(func() { os.Unsetenv("DEVTEAM_ADMIN_TOKEN") })

	called := false
	guarded := AdminGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	// Missing token.
	req := httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	req.RemoteAddr = "10.0.0.5:54321"
	w := httptest.NewRecorder()
	guarded.ServeHTTP(w, req)
	if called {
		t.Error("handler called for missing-token request (must not produce side effects)")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing-token status = %d, want 401", w.Code)
	}

	// Wrong token.
	called = false
	req = httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	req.RemoteAddr = "10.0.0.5:54321"
	req.Header.Set("X-Devteam-Admin-Token", "wrong-token")
	w = httptest.NewRecorder()
	guarded.ServeHTTP(w, req)
	if called {
		t.Error("handler called for wrong-token request")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong-token status = %d, want 401", w.Code)
	}
}

// TestGuard_FailClosed_WhenEnvUnset verifies the fail-closed behavior: when
// DEVTEAM_ADMIN_TOKEN is unset and the request is non-localhost, the guard
// rejects with 401 (no accidental open writes — FR-ROUTE-03, R-AUTH-ABSENT).
func TestGuard_FailClosed_WhenEnvUnset(t *testing.T) {
	os.Unsetenv("DEVTEAM_ADMIN_TOKEN")

	called := false
	guarded := AdminGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/repos", nil)
	req.RemoteAddr = "10.0.0.5:54321"
	// No header set — env unset + non-localhost → fail-closed.
	w := httptest.NewRecorder()
	guarded.ServeHTTP(w, req)

	if called {
		t.Error("handler called for non-localhost request when token unset (fail-closed violated)")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (fail-closed)", w.Code)
	}
}

// TestGuard_PluggableMiddleware verifies AdminGuard is an http.Handler
// middleware (wraps an http.Handler, returns an http.Handler) — the
// FR-ROUTE-04 reviewer check that RBAC can swap it without route changes
// (NFR-MAINT-03).
func TestGuard_PluggableMiddleware(t *testing.T) {
	var _ http.Handler = AdminGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	// If this compiles, AdminGuard is a middleware. The assertion is the
	// type assignment above; no runtime check needed.
}