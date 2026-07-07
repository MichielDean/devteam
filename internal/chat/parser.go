package chat

import (
	"fmt"
	"strings"
)

// ─── Tool-call / citation tail-parser (U-CH-8, F3 fix) ─────────────────────
//
// The expert emits tool-call proposals and citations inside delimited blocks
// in its streamed output. The parser extracts these blocks from the stream
// so the chat backend can route tool-calls through the CLI-proxy confirm gate
// and render citations as first-class UI elements (FR-G2-4, FR-G2-5).
//
// The delimiter format (decided here, unit-tested against synthetic streams):
//
//   <tool-call>
//   verb: <verb>
//   args: <args...>
//   </tool-call>
//
//   <citations>
//   - file: <path>
//     section: <section>
//     lines: <start>-<end>      (optional)
//   </citations>
//
// Parsing is tolerant: unknown delimiters are passed through as plain text;
// malformed blocks (missing closing delimiter) are emitted as-is at stream end
// rather than dropped silently (so the user sees what the expert wrote).

// ToolCall is one parsed <tool-call> block.
type ToolCall struct {
	Verb string
	Args string
}

// Citation is one parsed <citations> entry.
type Citation struct {
	File    string `json:"file"`
	Section string `json:"section,omitempty"`
	Lines   string `json:"lines,omitempty"` // "start-end" or "start"
}

// ParseResult is what ParseStream returns: the cleaned text (with blocks
// removed), any tool-calls found, and any citations found.
type ParseResult struct {
	Text       string
	ToolCalls  []ToolCall
	Citations  []Citation
}

// ParseStream extracts <tool-call> and <citations> blocks from the expert's
// streamed output. The returned Text has the blocks removed; the blocks'
// contents are parsed into ToolCalls/Citations. Malformed/unclosed blocks
// are left in the text verbatim (the user sees them).
func ParseStream(stream string) ParseResult {
	text := stream
	var toolCalls []ToolCall
	var citations []Citation

	// Extract <tool-call>...</tool-call> blocks.
	text, toolCallBlocks := extractBlocks(text, "<tool-call>", "</tool-call>", parseToolCallBody)
	toolCalls = toolCallBlocks

	// Extract <citations>...</citations> blocks. The parser returns a wrapper
	// (CitationsBlock) so the generic extractBlocks infers T correctly.
	text, citBlocks := extractBlocks(text, "<citations>", "</citations>", parseCitationsBody)
	for _, cb := range citBlocks {
		citations = append(citations, cb.Entries...)
	}

	return ParseResult{Text: text, ToolCalls: toolCalls, Citations: citations}
}

// extractBlocks finds all <open>...</close> blocks in text, parses each via
// the parser func, and returns the text with well-formed blocks removed plus
// the slice of parsed results. Malformed (unclosed) blocks are left in text.
func extractBlocks[T any](text, open, closeTag string, parser func(body string) (T, bool)) (string, []T) {
	var results []T
	var out strings.Builder
	rest := text
	for {
		idx := strings.Index(rest, open)
		if idx < 0 {
			out.WriteString(rest)
			break
		}
		// Write everything before the block.
		out.WriteString(rest[:idx])
		after := rest[idx+len(open):]
		endIdx := strings.Index(after, closeTag)
		if endIdx < 0 {
			// Unclosed block — emit verbatim and stop (the user sees it).
			out.WriteString(rest[idx:])
			break
		}
		body := after[:endIdx]
		if parsed, ok := parser(body); ok {
			results = append(results, parsed)
			// Skip the block + its delimiters. The block is removed from text.
			rest = after[endIdx+len(closeTag):]
		} else {
			// Malformed body — emit verbatim and continue.
			out.WriteString(rest[idx : idx+len(open)+endIdx+len(closeTag)])
			rest = after[endIdx+len(closeTag):]
		}
	}
	return out.String(), results
}

// parseToolCallBody parses the body of a <tool-call> block:
//   verb: <verb>
//   args: <args...>
// Returns ok=false if the body is malformed (no verb line).
func parseToolCallBody(body string) (ToolCall, bool) {
	var tc ToolCall
	lines := strings.Split(strings.TrimSpace(body), "\n")
	var argsLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "verb:") {
			tc.Verb = strings.TrimSpace(strings.TrimPrefix(line, "verb:"))
		} else if strings.HasPrefix(line, "args:") {
			argsLines = append(argsLines, strings.TrimSpace(strings.TrimPrefix(line, "args:")))
		}
		// Additional unstructured lines (continuation of args) are appended.
	}
	if tc.Verb == "" {
		return ToolCall{}, false
	}
	tc.Args = strings.Join(argsLines, " ")
	return tc, true
}

// parseCitationsBody parses the body of a <citations> block as a simple YAML-ish
// list of {file, section, lines?} entries. Returns ok=false if no entries parse.
func parseCitationsBody(body string) (CitationsBlock, bool) {
	var cits []Citation
	lines := strings.Split(strings.TrimSpace(body), "\n")
	var cur *Citation
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			// New entry starts. File is the first field of the list item.
			if cur != nil {
				cits = append(cits, *cur)
			}
			cur = &Citation{}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if strings.HasPrefix(rest, "file:") {
				cur.File = strings.TrimSpace(strings.TrimPrefix(rest, "file:"))
			} else {
				// `- file: ...` is the expected form; tolerate `- <path>` too.
				cur.File = rest
			}
		} else if cur != nil {
			// Continuation field of the current entry.
			k, v, ok := splitField(trimmed)
			if !ok {
				continue
			}
			switch k {
			case "file":
				cur.File = v
			case "section":
				cur.Section = v
			case "lines":
				cur.Lines = v
			}
		}
	}
	if cur != nil {
		cits = append(cits, *cur)
	}
	if len(cits) == 0 {
		return CitationsBlock{}, false
	}
	return CitationsBlock{Entries: cits}, true
}

// CitationsBlock is the generic-T wrapper for extractBlocks when parsing a
// <citations> block (which yields a slice, not a single item).
type CitationsBlock struct {
	Entries []Citation
}

func splitField(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// FormatCitationsJSON renders citations as the jsonb the chat_messages table
// stores ([]byte). Returns nil if no citations.
func FormatCitationsJSON(cits []Citation) []byte {
	if len(cits) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, c := range cits {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf(`{"file":%q`, c.File))
		if c.Section != "" {
			b.WriteString(fmt.Sprintf(`,"section":%q`, c.Section))
		}
		if c.Lines != "" {
			b.WriteString(fmt.Sprintf(`,"lines":%q`, c.Lines))
		}
		b.WriteByte('}')
	}
	b.WriteByte(']')
	return []byte(b.String())
}