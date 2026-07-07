package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ─── RAG indexer (U-CH-5, F2 indexer half) ───────────────────────────────
//
// BuildRAGIndex walks the corpus declared in roles/expert/knowledge.yaml,
// chunks each file by markdown headers, and writes a derived JSON index to
// .devteam/rag-index.json. The index is rebuildable on corpus change (DR-4);
// corpus files are NOT modified (the index is a derived artifact).
//
// The on-disk JSON shape is pinned by a test (U-CH-5 → U-CH-6 contract):
//   {
//     "built_at": "<RFC3339>",
//     "corpus_mtime": "<RFC3339 of newest corpus file>",
//     "chunks": [
//       {"id": "...", "file": "AGENTS.md", "section": "Phases", "body": "...", "lines": "1-30"}
//     ]
//   }
//
// Retrieve (U-CH-6) loads this exact shape. If the on-disk shape changes,
// the U-CH-6 test catches it (dependency-dag §5 edge U-CH-5→U-CH-6).

// Chunk is one indexed unit: a section of a corpus file.
type Chunk struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Section string `json:"section"`
	Lines   string `json:"lines,omitempty"`
	Body    string `json:"body"`
}

// RAGIndex is the on-disk index shape.
type RAGIndex struct {
	BuiltAt     string  `json:"built_at"`
	CorpusMTime string  `json:"corpus_mtime"`
	Chunks      []Chunk `json:"chunks"`
}

// KnowledgeManifest is a minimal decoder for roles/expert/knowledge.yaml.
// Only the corpus + skills fields are read by the indexer.
type KnowledgeManifest struct {
	Corpus []struct {
		Path string `yaml:"path" json:"path"`
	} `yaml:"corpus"`
	Skills []struct {
		File        string `yaml:"file" json:"file"`
		Description string `yaml:"description" json:"description"`
	} `yaml:"skills"`
}

// BuildRAGIndex walks the manifest's corpus, chunks each file, and writes the
// index to outPath. Returns the built index (so tests can assert without
// re-reading). baseDir is the repo root (where AGENTS.md, roles/, etc. live).
func BuildRAGIndex(baseDir, manifestPath, outPath string) (*RAGIndex, error) {
	manifest, err := LoadKnowledgeManifest(manifestPath)
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
	var newestMTime time.Time
	for _, entry := range manifest.Corpus {
		absPath := filepath.Join(baseDir, entry.Path)
		info, err := os.Stat(absPath)
		if err != nil {
			// Missing corpus file → skip (NFR-REL-2: missing index degrades gracefully).
			continue
		}
		if info.ModTime().After(newestMTime) {
			newestMTime = info.ModTime()
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}
		fileChunks := chunkMarkdown(string(data), entry.Path)
		chunks = append(chunks, fileChunks...)
	}
	idx := &RAGIndex{
		BuiltAt:     time.Now().UTC().Format(time.RFC3339),
		CorpusMTime: newestMTime.UTC().Format(time.RFC3339),
		Chunks:      chunks,
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return nil, fmt.Errorf("creating index dir: %w", err)
	}
	out, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling index: %w", err)
	}
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		return nil, fmt.Errorf("writing index: %w", err)
	}
	return idx, nil
}

// LoadKnowledgeManifest reads roles/expert/knowledge.yaml.
func LoadKnowledgeManifest(path string) (*KnowledgeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading knowledge manifest %s: %w", path, err)
	}
	var m KnowledgeManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing knowledge manifest: %w", err)
	}
	return &m, nil
}

// chunkMarkdown splits a markdown file into chunks by H1/H2 headers. Each
// chunk carries its section title and (rough) line range. Content before the
// first header becomes a chunk with section = "<intro>".
func chunkMarkdown(content, filePath string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var current []string
	var currentSection string
	var sectionStart int
	currentSection = "<intro>"
	sectionStart = 1

	flush := func(endLine int) {
		if len(current) == 0 {
			return
		}
		body := strings.TrimSpace(strings.Join(current, "\n"))
		if body == "" {
			current = nil
			return
		}
		chunks = append(chunks, Chunk{
			ID:      chunkID(filePath, currentSection),
			File:    filePath,
			Section: currentSection,
			Lines:   fmt.Sprintf("%d-%d", sectionStart, endLine),
			Body:    body,
		})
		current = nil
	}

	for i, line := range lines {
		lineNum := i + 1
		// H1 or H2 starts a new chunk.
		if isHeaderLine(line) {
			flush(lineNum - 1)
			currentSection = strings.TrimSpace(strings.TrimLeft(line, "# "))
			sectionStart = lineNum
		}
		current = append(current, line)
	}
	flush(len(lines))
	return chunks
}

var headerRe = regexp.MustCompile(`^#{1,2}\s+\S`)

func isHeaderLine(line string) bool {
	return headerRe.MatchString(line)
}

func chunkID(file, section string) string {
	// Stable id: file + section, slugified. Collisions are acceptable (two
	// chunks with the same section in the same file get distinct line ranges).
	slug := strings.NewReplacer("/", "_", " ", "_", ".", "_").Replace(file + "_" + section)
	return strings.ToLower(slug)
}

// ─── RAG retriever (U-CH-6, F2 retriever half) ───────────────────────────
//
// Retrieve loads the on-disk index and returns the top-k chunks whose body
// contains any of the query terms (case-insensitive substring match). This
// is a deliberately simple lexical retriever — the MVS does not require
// vector embeddings (the BV3 go/no-go could defer that to BP2). The on-disk
// JSON shape is pinned; if it changes, the U-CH-6 test catches it.
//
// Latency target: <500ms p95 (NFR-PER-2). Lexical retrieval over ~50 chunks
// is sub-millisecond on any reasonable hardware.

// Retrieve returns the top-k chunks matching the query.
func Retrieve(index *RAGIndex, query string, k int) []Chunk {
	if index == nil || len(index.Chunks) == 0 {
		return nil
	}
	if k <= 0 {
		k = 5
	}
	terms := tokenize(query)
	type scored struct {
		Chunk
		Score int
	}
	var results []scored
	for _, c := range index.Chunks {
		score := scoreChunk(c, terms)
		if score > 0 {
			results = append(results, scored{Chunk: c, Score: score})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		// Stable tiebreak: chunk id.
		return results[i].ID < results[j].ID
	})
	if len(results) > k {
		results = results[:k]
	}
	out := make([]Chunk, len(results))
	for i, r := range results {
		out[i] = r.Chunk
	}
	return out
}

func tokenize(query string) []string {
	query = strings.ToLower(query)
	words := strings.FieldsFunc(query, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '.' || r == '?' || r == '!' || r == ';' || r == ':'
	})
	// Deduplicate.
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		if len(w) < 2 { // skip single letters / short tokens
			continue
		}
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func scoreChunk(c Chunk, terms []string) int {
	body := strings.ToLower(c.Body)
	section := strings.ToLower(c.Section)
	score := 0
	for _, t := range terms {
		if strings.Contains(section, t) {
			score += 5 // section match is a strong signal
		}
		if strings.Contains(body, t) {
			score += 1
		}
	}
	return score
}

// LoadRAGIndex reads .devteam/rag-index.json. Returns nil + error if the
// file is missing or unparseable; callers handle the degraded case (NFR-REL-2).
func LoadRAGIndex(path string) (*RAGIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading rag index: %w", err)
	}
	var idx RAGIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing rag index: %w", err)
	}
	return &idx, nil
}