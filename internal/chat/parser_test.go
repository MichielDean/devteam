package chat

import (
	"encoding/json"
	"testing"
)

func TestParseStream_ToolCallExtracted(t *testing.T) {
	stream := "I'll create that feature for you.\n<tool-call>\nverb: feature create\nargs: --title \"My Feature\"\n</tool-call>\nDone."
	r := ParseStream(stream)
	if len(r.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool-call, got %d", len(r.ToolCalls))
	}
	tc := r.ToolCalls[0]
	if tc.Verb != "feature create" {
		t.Errorf("verb = %q, want 'feature create'", tc.Verb)
	}
	if tc.Args != `--title "My Feature"` {
		t.Errorf("args = %q, want '--title \"My Feature\"'", tc.Args)
	}
	if ContainsBlock(r.Text, "<tool-call>") {
		t.Errorf("text should not contain tool-call block: %q", r.Text)
	}
}

func TestParseStream_CitationsExtracted(t *testing.T) {
	stream := `The 5 phases are initialization, ideation, inception, construction, operation.
<citations>
- file: AGENTS.md
  section: Phases
- file: roles/architect/INSTRUCTIONS.md
  section: Stages Owned
  lines: 42-58
</citations>`
	r := ParseStream(stream)
	if len(r.Citations) != 2 {
		t.Fatalf("expected 2 citations, got %d: %+v", len(r.Citations), r.Citations)
	}
	if r.Citations[0].File != "AGENTS.md" || r.Citations[0].Section != "Phases" {
		t.Errorf("cit[0] = %+v", r.Citations[0])
	}
	if r.Citations[1].File != "roles/architect/INSTRUCTIONS.md" {
		t.Errorf("cit[1].File = %q", r.Citations[1].File)
	}
	if r.Citations[1].Lines != "42-58" {
		t.Errorf("cit[1].Lines = %q, want 42-58", r.Citations[1].Lines)
	}
}

func TestParseStream_MultipleToolCalls(t *testing.T) {
	stream := "<tool-call>\nverb: status\n</tool-call>\nthen\n<tool-call>\nverb: stages\n</tool-call>\n"
	r := ParseStream(stream)
	if len(r.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool-calls, got %d", len(r.ToolCalls))
	}
	if r.ToolCalls[0].Verb != "status" || r.ToolCalls[1].Verb != "stages" {
		t.Errorf("verbs = %q, %q", r.ToolCalls[0].Verb, r.ToolCalls[1].Verb)
	}
}

func TestParseStream_NoBlocks(t *testing.T) {
	stream := "Just a plain answer with no blocks."
	r := ParseStream(stream)
	if len(r.ToolCalls) != 0 || len(r.Citations) != 0 {
		t.Errorf("expected 0 blocks, got %d tool-calls / %d citations", len(r.ToolCalls), len(r.Citations))
	}
	if r.Text != stream {
		t.Errorf("text should be unchanged, got %q", r.Text)
	}
}

func TestParseStream_UnclosedBlockLeftVerbatim(t *testing.T) {
	stream := "text <tool-call>\nverb: status\n"
	r := ParseStream(stream)
	if len(r.ToolCalls) != 0 {
		t.Errorf("unclosed block should not parse as a tool-call, got %d", len(r.ToolCalls))
	}
	if !ContainsBlock(r.Text, "<tool-call>") {
		t.Errorf("unclosed block should remain in text: %q", r.Text)
	}
}

func TestParseStream_MalformedToolCallNoVerb(t *testing.T) {
	stream := "<tool-call>\nargs: --foo\n</tool-call>\n"
	r := ParseStream(stream)
	if len(r.ToolCalls) != 0 {
		t.Errorf("malformed (no verb) should not parse, got %d", len(r.ToolCalls))
	}
}

func TestFormatCitationsJSON(t *testing.T) {
	cits := []Citation{
		{File: "AGENTS.md", Section: "Phases"},
		{File: "roles/architect/INSTRUCTIONS.md", Section: "Stages Owned", Lines: "42-58"},
	}
	out := FormatCitationsJSON(cits)
	if out == nil {
		t.Fatal("expected non-nil jsonb")
	}
	var decoded []map[string]string
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json invalid: %v\n%s", err, out)
	}
	if len(decoded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(decoded))
	}
	if decoded[0]["file"] != "AGENTS.md" {
		t.Errorf("decoded[0].file = %q", decoded[0]["file"])
	}
	if decoded[1]["lines"] != "42-58" {
		t.Errorf("decoded[1].lines = %q", decoded[1]["lines"])
	}
}

func TestFormatCitationsJSON_NilWhenEmpty(t *testing.T) {
	if out := FormatCitationsJSON(nil); out != nil {
		t.Errorf("expected nil for empty, got %s", out)
	}
}

// ContainsBlock is a test helper.
func ContainsBlock(s, marker string) bool {
	return len(s) >= len(marker) && indexOf(s, marker) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}