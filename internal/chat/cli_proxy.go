package chat

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ─── CLI-proxy authorization boundary (C7, D3, FR-CL-1..4) ────────────────
//
// The CLI-proxy is the authorization boundary between the expert (which
// proposes devteam CLI ops) and the actual subprocess execution. The proxy:
//
//   - Validates the verb against an allowlist (NFR-SEC-1 — server-side).
//   - Classifies the verb as read-only / mutating / destructive.
//   - Read-only verbs run immediately (FR-CL-1).
//   - Mutating verbs require user confirm UNLESS trust_mode is on (FR-CL-2).
//   - Destructive verbs ALWAYS require confirm naming the consequence (FR-CL-3).
//   - Rejects any non-allowlist verb (FR-CL-4, NFR-SEC-1).
//   - Runs via exec.Command("devteam", verb, args...) — NO shell (NFR-SEC-2).
//   - Emits a chat_cli_exec audit event for every op (incl. rejected — NFR-SEC-6).

// VerbClassification is the safety class of a CLI verb.
type VerbClassification string

const (
	ClassReadOnly    VerbClassification = "read-only"
	ClassMutating    VerbClassification = "mutating"
	ClassDestructive VerbClassification = "destructive"
)

// allowedVerb is one entry in the allowlist.
type allowedVerb struct {
	verb        string
	class        VerbClassification
	consequence  string // for destructive: the consequence string shown in confirm
	// signalStyle: if true, the verb is a "signal <id> <outcome>" form — the
	// match skips the feature-id slot (position 1) and checks the outcome
	// keyword at position 2. This accommodates the devteam CLI's shape where
	// `signal <feature-id> pass` has the subcommand after the id.
	signalStyle bool
}

// verbPattern is a constructor shorthand for normal (non-signal) verbs.
func verbPattern(v string) allowedVerb {
	return allowedVerb{verb: v}
}

// FindVerb returns the allowlist entry for a verb prefix, or nil if not allowed.
// The match accommodates two shapes:
//   - "verb args..." — the verb prefix matches (e.g. "feature create --title X").
//   - "signal <id> <outcome>" — signalStyle verbs skip the id slot and match
//     the outcome keyword at position 2 (e.g. "signal abc-123 pass").
func FindVerb(command string) *allowedVerb {
	command = strings.TrimSpace(command)
	var best *allowedVerb
	for i := range allowlist {
		v := &allowlist[i]
		if v.signalStyle {
			if matchSignal(command, v.verb) {
				if best == nil || len(v.verb) > len(best.verb) {
					best = v
				}
			}
		} else {
			if command == v.verb || strings.HasPrefix(command, v.verb+" ") {
				if best == nil || len(v.verb) > len(best.verb) {
					best = v
				}
			}
		}
	}
	return best
}

// matchSignal checks if command is "signal <id> <outcome>" where outcome
// matches the verb's outcome keyword (the part after "signal " in v.verb).
func matchSignal(command, verb string) bool {
	// verb is "signal <outcome>"
	outcome := strings.TrimSpace(strings.TrimPrefix(verb, "signal "))
	parts := strings.Fields(command)
	if len(parts) < 3 {
		return false
	}
	if parts[0] != "signal" {
		return false
	}
	// parts[1] is the feature-id slot (wildcard). parts[2] is the outcome.
	if parts[2] != outcome {
		return false
	}
	return true
}

// Allowlist is the set of devteam verbs the expert may propose. The exact
// list is bounded by FR-CL-1..3 and specified in 2.4; this is the v1 list.
//
// The matching is on the *verb prefix* — the first words that identify the
// command. Many devteam verbs take a feature-id as the first arg followed by
// a subcommand (e.g. `signal <id> pass`); the allowlist matches the verb shape
// up to and including the subcommand keyword, with the feature-id slot
// treated as a wildcard.
//
// Read-only: status, list, info, stages, audit, artifacts (no side effects).
// Mutating: feature create, signal pass/recirculate/needs_feedback, run-stage,
//   answer questions, artifact submit (safe non-destructive mutations).
// Destructive: cancel feature, delete repo (always confirm + consequence).
var allowlist = []allowedVerb{
	// Read-only — run without confirm (FR-CL-1, SC7).
	{verb: "feature status", class: ClassReadOnly},
	{verb: "feature info", class: ClassReadOnly},
	{verb: "stages", class: ClassReadOnly},
	{verb: "audit", class: ClassReadOnly},
	{verb: "artifacts", class: ClassReadOnly},
	{verb: "artifact get", class: ClassReadOnly},
	{verb: "repos list", class: ClassReadOnly},
	{verb: "repo list", class: ClassReadOnly},
	{verb: "questions list", class: ClassReadOnly},

	// Mutating — confirm unless trust_mode on (FR-CL-2, SC8).
	{verb: "feature create", class: ClassMutating},
	// signal <id> pass/recirculate/needs_feedback/failed — the outcome keyword
	// is at position 2 (after the feature-id). matchSignal handles this shape.
	{verb: "signal pass", class: ClassMutating, signalStyle: true},
	{verb: "signal recirculate", class: ClassMutating, signalStyle: true},
	{verb: "signal needs_feedback", class: ClassMutating, signalStyle: true},
	{verb: "signal failed", class: ClassMutating, signalStyle: true},
	{verb: "run-stage", class: ClassMutating},
	{verb: "questions answer", class: ClassMutating},
	{verb: "artifact submit", class: ClassMutating},
	{verb: "questions ask", class: ClassMutating},

	// Destructive — ALWAYS confirm + consequence (FR-CL-3, SC9).
	{verb: "feature cancel", class: ClassDestructive, consequence: "Cancels the feature; this cannot be undone."},
	{verb: "cancel feature", class: ClassDestructive, consequence: "Cancels the feature; this cannot be undone."},
	{verb: "repo delete", class: ClassDestructive, consequence: "Deletes the repo registration; this cannot be undone."},
	{verb: "delete repo", class: ClassDestructive, consequence: "Deletes the repo registration; this cannot be undone."},
}

// IsAllowed reports whether a command's verb is on the allowlist.
func IsAllowed(command string) bool {
	return FindVerb(command) != nil
}

// Classify returns the classification for a command. Returns ClassReadOnly
// for unknown verbs (the proxy rejects before classification matters, but
// this default is safe).
func Classify(command string) VerbClassification {
	v := FindVerb(command)
	if v == nil {
		return ClassReadOnly
	}
	return v.class
}

// Consequence returns the consequence string for a destructive verb, or "".
func Consequence(command string) string {
	v := FindVerb(command)
	if v == nil {
		return ""
	}
	return v.consequence
}

// ─── Proposal lifecycle (transient, server-side map) ──────────────────────
//
// A CliProposal is created when the expert proposes an op. It lives in a
// server map with a short TTL until the user confirms or rejects (or it
// expires). Proposals are NOT persisted (app-design §5.3 implicit assumption
// 3 — server restart loses in-flight proposals; the expert re-proposes).

// CliProposal is a pending CLI op awaiting user decision.
type CliProposal struct {
	ID            string
	SessionID     string
	Command       string
	Classification VerbClassification
	Consequence   string
	ConfirmToken  string
	Args          []string // parsed args for exec.Command
	CreatedAt     time.Time
	ExpiresAt     time.Time
}

// ProposalStore is the in-memory transient proposal map.
type ProposalStore struct {
	mu        sync.Mutex
	proposals map[string]*CliProposal
	ttl       time.Duration
}

// NewProposalStore creates a store with the given proposal TTL (default 5m).
func NewProposalStore(ttl time.Duration) *ProposalStore {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ProposalStore{
		proposals: map[string]*CliProposal{},
		ttl:       ttl,
	}
}

// Create validates + stores a proposal. Returns the proposal (with token) or
// an error if the verb is not allowed. The command is the full
// "verb args..." string the expert proposed.
func (s *ProposalStore) Create(sessionID, command string, trustMode bool) (*CliProposal, error) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return nil, fmt.Errorf("empty command")
	}
	v := FindVerb(cmd)
	if v == nil {
		return nil, fmt.Errorf("verb_not_allowed: %q is not on the allowlist", firstWord(cmd))
	}
	// Parse into argv for exec.Command (no shell — NFR-SEC-2). For signal-style
	// verbs the full command IS the argv ("signal <id> <outcome> ..."). For
	// other verbs the command is "<verb> <args...>" — strip the verb prefix.
	var args []string
	if v.signalStyle {
		args = parseArgs(cmd)
	} else {
		args = parseArgs(strings.TrimPrefix(strings.TrimSpace(cmd), v.verb))
	}
	// Read-only → no proposal needed; the caller should run it immediately.
	// Mutating + trust_mode → no proposal needed; run immediately.
	// Mutating without trust_mode → proposal.
	// Destructive → ALWAYS proposal (regardless of trust_mode).
	if v.class == ClassReadOnly {
		return &CliProposal{
			ID:             newProposalID(),
			SessionID:      sessionID,
			Command:        cmd,
			Classification: v.class,
			Args:           args,
			CreatedAt:      time.Now().UTC(),
			ExpiresAt:      time.Now().UTC().Add(s.ttl),
		}, nil
	}
	if v.class == ClassMutating && trustMode {
		return &CliProposal{
			ID:             newProposalID(),
			SessionID:      sessionID,
			Command:        cmd,
			Classification: v.class,
			Args:           args,
			CreatedAt:      time.Now().UTC(),
			ExpiresAt:      time.Now().UTC().Add(s.ttl),
		}, nil
	}
	p := &CliProposal{
		ID:             newProposalID(),
		SessionID:      sessionID,
		Command:        cmd,
		Classification: v.class,
		Consequence:    v.consequence,
		ConfirmToken:   newProposalID(),
		Args:           args,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(s.ttl),
	}
	s.mu.Lock()
	s.proposals[p.ID] = p
	s.mu.Unlock()
	return p, nil
}

// Get returns a pending proposal by id, or nil if not found / expired.
func (s *ProposalStore) Get(id string) *CliProposal {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.proposals[id]
	if !ok {
		return nil
	}
	if time.Now().UTC().After(p.ExpiresAt) {
		delete(s.proposals, id)
		return nil
	}
	return p
}

// Resolve removes a proposal (after confirm or reject) and returns it.
func (s *ProposalStore) Resolve(id string) *CliProposal {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.proposals[id]
	if !ok {
		return nil
	}
	delete(s.proposals, id)
	return p
}

// NeedsConfirm reports whether a proposal requires user confirmation.
func (p *CliProposal) NeedsConfirm() bool {
	return p.ConfirmToken != ""
}

// ─── Execution (no shell — NFR-SEC-2) ─────────────────────────────────────
//
// Execute runs the proposal via exec.Command("devteam", verb, args...). The
// verb + args are already parsed; no shell interpolation. Returns stdout,
// stderr, exit code.
//
// The caller is responsible for the audit event (so it can record confirmed
// state). Execute itself is just the subprocess invocation.

// ExecResult is the outcome of a CLI op.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Execute runs the proposal. verbArgs is the full argv (verb first).
func Execute(verbArgs []string) (*ExecResult, error) {
	if len(verbArgs) == 0 {
		return nil, fmt.Errorf("no command to execute")
	}
	cmd := exec.Command("devteam", verbArgs...)
	out, err := cmd.CombinedOutput()
	result := &ExecResult{Stdout: string(out)}
	if exitErr, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitErr.ExitCode()
		result.Stderr = string(exitErr.Stderr)
	} else if err != nil {
		// Process not started / not found.
		return result, fmt.Errorf("exec devteam: %w", err)
	}
	return result, nil
}

// parseArgs splits a command tail into argv. Simple split on whitespace with
// double-quote handling. This is NOT shell parsing — it's argv splitting for
// exec.Command (NFR-SEC-2 — args are never interpolated into a shell string).
func parseArgs(tail string) []string {
	tail = strings.TrimSpace(tail)
	if tail == "" {
		return nil
	}
	var args []string
	var cur strings.Builder
	inQuote := false
	for _, r := range tail {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				cur.WriteRune(r)
			} else if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

// newProposalID returns a short unique id. Not a security boundary — just
// disambiguation. Uses unix nanos + a counter for uniqueness.
var proposalCounter uint64
var proposalMu sync.Mutex

func newProposalID() string {
	proposalMu.Lock()
	proposalCounter++
	c := proposalCounter
	proposalMu.Unlock()
	return fmt.Sprintf("p-%d-%d", time.Now().UTC().UnixNano(), c)
}