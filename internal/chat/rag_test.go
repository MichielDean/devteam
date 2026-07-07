package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRAGIndex_ChunksCorpus(t *testing.T) {
	tmp := t.TempDir()
	// Minimal corpus files.
	os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("# AIDLC\n\n5 phases.\n\n## Phases\n\ninit, ideation, inception, construction, operation.\n"), 0644)
	os.WriteFile(filepath.Join(tmp, "roles/architect/INSTRUCTIONS.md"), []byte("# Architect\n\nDesigns things.\n\n## Stages Owned\n\nApp Design.\n"), 0644)
	// Manifest
	manifest := tmp + "/knowledge.yaml"
	os.MkdirAll(filepath.Dir(manifest), 0755)
	os.WriteFile(manifest, []byte("corpus:\n  - path: AGENTS.md\n  - path: roles/architect/INSTRUCTIONS.md\n"), 0644)

	outPath := tmp + "/.devteam/rag-index.json"
	idx, err := BuildRAGIndex(tmp, manifest, outPath)
	if err != nil {
		t.Fatalf("BuildRAGIndex: %v", err)
	}
	if len(idx.Chunks) < 2 {
		t.Fatalf("expected ≥2 chunks, got %d", len(idx.Chunks))
	}
	// Each chunk carries its source file (FR-K-1, SC1/SC2 citation support).
	for _, c := range idx.Chunks {
		if c.File == "" {
			t.Error("chunk missing file")
		}
		if c.Section == "" {
			t.Error("chunk missing section")
		}
	}
	// Index written to disk
	if _, err := os.Stat(outPath); err != nil {
		t.Errorf("index not written: %v", err)
	}
}

func TestBuildRAGIndex_MissingCorpusFileSkipped(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("# A\n\nbody"), 0644)
	manifest := tmp + "/knowledge.yaml"
	os.WriteFile(manifest, []byte("corpus:\n  - path: AGENTS.md\n  - path: does-not-exist.md\n"), 0644)
	outPath := tmp + "/.devteam/rag-index.json"
	idx, err := BuildRAGIndex(tmp, manifest, outPath)
	if err != nil {
		t.Fatalf("BuildRAGIndex missing file: %v", err)
	}
	// Only the present file's chunks
	for _, c := range idx.Chunks {
		if c.File == "does-not-exist.md" {
			t.Error("missing corpus file should be skipped, not chunked")
		}
	}
}

func TestRetrieve_ReturnsMatchingChunks(t *testing.T) {
	idx := &RAGIndex{Chunks: []Chunk{
		{ID: "a", File: "AGENTS.md", Section: "Phases", Body: "initialization ideation inception construction operation"},
		{ID: "b", File: "roles/architect/INSTRUCTIONS.md", Section: "Stages Owned", Body: "App Design, NFR Design"},
		{ID: "c", File: "roles/developer/INSTRUCTIONS.md", Section: "Stages Owned", Body: "Code Generation, Reverse Engineering"},
	}}
	// Query about phases → should rank the AGENTS.md chunk first.
	results := Retrieve(idx, "what are the 5 phases", 5)
	if len(results) == 0 {
		t.Fatal("expected at least one match")
	}
	if results[0].File != "AGENTS.md" {
		t.Errorf("top result file = %q, want AGENTS.md", results[0].File)
	}
}

func TestRetrieve_TopKLimit(t *testing.T) {
	idx := &RAGIndex{Chunks: []Chunk{
		{ID: "a", File: "a.md", Section: "design", Body: "design design design"},
		{ID: "b", File: "b.md", Section: "design", Body: "design design"},
		{ID: "c", File: "c.md", Section: "design", Body: "design"},
	}}
	results := Retrieve(idx, "design", 2)
	if len(results) != 2 {
		t.Errorf("expected 2 results (k=2), got %d", len(results))
	}
}

func TestRetrieve_EmptyIndexReturnsNil(t *testing.T) {
	if r := Retrieve(&RAGIndex{}, "anything", 5); r != nil {
		t.Errorf("expected nil for empty index, got %v", r)
	}
	if r := Retrieve(nil, "anything", 5); r != nil {
		t.Errorf("expected nil for nil index, got %v", r)
	}
}

func TestLoadRAGIndex_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "AGENTS.md"), []byte("# AIDLC\n\n5 phases.\n"), 0644)
	manifest := tmp + "/knowledge.yaml"
	os.WriteFile(manifest, []byte("corpus:\n  - path: AGENTS.md\n"), 0644)
	outPath := tmp + "/.devteam/rag-index.json"
	idx, err := BuildRAGIndex(tmp, manifest, outPath)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Load it back — the on-disk shape must round-trip (U-CH-5→U-CH-6 contract).
	loaded, err := LoadRAGIndex(outPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Chunks) != len(idx.Chunks) {
		t.Errorf("round-trip chunk count: built=%d loaded=%d", len(idx.Chunks), len(loaded.Chunks))
	}
	// And retrieve against the loaded index.
	results := Retrieve(loaded, "phases", 5)
	if len(results) == 0 {
		t.Error("expected retrieve against loaded index to return the phases chunk")
	}
}

func TestLoadRAGIndex_MissingFileReturnsError(t *testing.T) {
	_, err := LoadRAGIndex("/does/not/exist.json")
	if err == nil {
		t.Error("expected error for missing index (NFR-REL-2 — caller degrades)")
	}
}