package api

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestSmoke_ServerStartsAndResponds verifies the server starts on a random port
// and key API endpoints respond. This is the smoke-level test (QG-1 gate):
// "server starts and responds without panicking".
func TestSmoke_ServerStartsAndResponds(t *testing.T) {
	s, _ := setupTestServer(t)

	// Replace the address with a random free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	s.httpServer.Addr = addr
	ln.Close()

	// Start the server in a goroutine.
	go func() {
		_ = s.Start()
	}()

	// Give it a moment to bind.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("server did not start within 3s: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	}
	defer s.Shutdown(context.Background())

	baseURL := "http://" + addr
	client := &http.Client{Timeout: 5 * time.Second}

	// Smoke: GET /api/features should return 200 with a JSON array.
	resp, err := client.Get(baseURL + "/api/features")
	if err != nil {
		t.Fatalf("GET /api/features: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /api/features: status = %d, want 200", resp.StatusCode)
	}

	// Smoke: GET /api/repos should return 200.
	resp2, err := client.Get(baseURL + "/api/repos")
	if err != nil {
		t.Fatalf("GET /api/repos: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("GET /api/repos: status = %d, want 200", resp2.StatusCode)
	}

	// Smoke: GET /api/settings/defaults should return 200.
	resp3, err := client.Get(baseURL + "/api/settings/defaults")
	if err != nil {
		t.Fatalf("GET /api/settings/defaults: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("GET /api/settings/defaults: status = %d, want 200", resp3.StatusCode)
	}

	// Smoke: GET /api/config/providers should return 200 (multi-provider feature).
	resp4, err := client.Get(baseURL + "/api/config/providers")
	if err != nil {
		t.Fatalf("GET /api/config/providers: %v", err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("GET /api/config/providers: status = %d, want 200", resp4.StatusCode)
	}
}