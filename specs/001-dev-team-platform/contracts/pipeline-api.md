# Pipeline API Contract

**Feature**: 001-dev-team-platform

## Run Phase

### Request

```go
type RunRequest struct {
    FeatureID string
    Phase     Phase    // If empty, run next phase
    DryRun    bool     // Evaluate gate without dispatching agent
}
```

### Response

```go
type RunResponse struct {
    FeatureID  string
    Phase      Phase
    GateResult *GateResult
    Artifacts  []string
}
```

### CLI

```bash
devteam run 001-user-auth
devteam run 001-user-auth --dry-run
```

### Behavior

1. Load feature state from `.devteam-state.yaml`
2. Determine current phase
3. Load phase configuration from `devteam.yaml`
4. Dispatch the role(s) for the current phase with:
   - Role INSTRUCTIONS.md
   - AIDLC phase rules
   - Feature spec context
   - Any active extensions based on priority
5. Collect output artifacts
6. Update feature state

---

## Evaluate Gate

### Request

```bash
devteam gate 001-user-auth
```

### Response

```go
type GateResult struct {
    Phase       Phase
    Passed      bool
    RequiredArts []ArtifactCheck
    Checks      []CheckResult
    EvaluatedAt time.Time
}
```

### Behavior

1. Load feature state
2. Determine current phase
3. Check required artifacts for the current phase gate
4. Run validation checks
5. Return pass/fail with details

### Exit Codes

- 0: Gate passed
- 1: Gate failed (missing artifacts or failed checks)

---

## Status

### CLI

```bash
devteam status
```

### Output

```
Dev Team Status:
======================================================================
  ID                                  Phase        Priority  Status
----------------------------------------------------------------------
  001-user-auth                       inception    2         in_progress
  002-api-rate-limit                  planning     2         in_progress
```