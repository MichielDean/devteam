package api

import (
	"testing"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/pipeline"
	"github.com/MichielDean/devteam/internal/spec"
	"net/http/httptest"
	"net/http"
	"io"
)

// FR-011 invariant (C17/ADR-010): the management console pages
// (Dashboard, FeatureDetail, KnowledgePage) render unchanged with NO
// provider info. The provider picker is chat-route-only. This test verifies
// the invariant at the API level: the /api/features endpoints + /api/repos
// do not surface provider config, even when providers are configured.
//
// Uses devteam_test_chat (not devteam_test_db) to avoid colliding with the
// db package's tests when `go test ./...` runs packages in parallel —
// TruncateAllTables on a shared DB wipes data mid-test.
func TestFR011_ManagementConsoleHasNoProviderInfo(t *testing.T) {
	const fr011TestDSN = "host=localhost port=5432 user=devteam password=devteam dbname=devteam_test_chat sslmode=disable"
	database, err := db.Open(db.Config{DSN: fr011TestDSN}, fr011TestDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	db.TruncateAllTables(database)
	t.Cleanup(func() { database.Close() })

	sp := spec.NewSpecProvider("/tmp")
	pipe := pipeline.NewPipeline(nil, sp)
	qs := feature.NewDBQuestionStore(database)
	s := NewServer(":0", sp, pipe, nil, qs, database)
	// Configure providers — the invariant is that this DOES NOT leak into
	// the management console endpoints.
	s.SetChatConfig(&config.Config{
		Providers: config.ProviderList{
			{Name: "ollama", BaseURL: "http://localhost:11434/v1", Model: "glm-5.2:cloud", Adapter: "openai"},
			{Name: "openai", BaseURL: "https://api.openai.com/v1", Model: "gpt-4o", Adapter: "openai"},
		},
	}, nil)
	ts := httptest.NewServer(s.httpServer.Handler)
	defer ts.Close()

	// The management console endpoints must not contain provider names/models.
	cases := []struct {
		name string
		path string
	}{
		{"features list", "/api/features"},
		{"repos list", "/api/repos"},
	}
	for _, c := range cases {
		resp, err := http.Get(ts.URL + c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)
		// The provider names/models must NOT appear in the management console.
		for _, forbidden := range []string{"gpt-4o", "openai", "glm-5.2:cloud"} {
			if contains(bodyStr, forbidden) {
				t.Errorf("%s: body contains provider info %q (FR-011 invariant violated): %s", c.name, forbidden, bodyStr)
			}
		}
	}

	// The chat providers endpoint DOES surface them (picker lives in chat).
	resp, err := http.Get(ts.URL + "/api/chat/providers")
	if err != nil {
		t.Fatalf("chat providers: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !contains(string(body), "gpt-4o") {
		t.Errorf("chat providers endpoint should surface provider info, got: %s", body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOfStr(s, sub) >= 0
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}