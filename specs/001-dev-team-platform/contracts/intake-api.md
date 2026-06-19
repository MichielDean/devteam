# Intake API Contract

**Feature**: 001-dev-team-platform

## Loose Idea Intake

### Request

```go
type IntakeRequest struct {
    Type     IntakeType  // LooseIdea or ExternalSpec
    Content  string      // Idea text or path to spec document
    Priority int         // 1, 2, or 3
    Repos    []string   // Initial repo list (PM may expand)
}
```

### Response

```go
type IntakeResponse struct {
    FeatureID string
    SpecPath  string   // Path to specs/NNN-*/ directory
    Artifacts []string // List of generated artifact paths
}
```

### CLI

```bash
devteam intake --type loose --text "We need user auth" --priority 2
```

### Behavior

1. PM role reads the loose idea text
2. PM explores and clarifies: asks structured questions about scope, acceptance criteria, affected repos
3. PM generates: `spec.md`, `acceptance.md`, `repos.yaml`
4. Feature is created in `inception` phase status

### Validation

- `Type` must be `loose_idea` or `external_spec`
- `Content` must be non-empty
- `Priority` must be 1, 2, or 3

---

## External Spec Intake

### Request

Same `IntakeRequest` with `Type = ExternalSpec` and `Content` containing the document path or text.

### Response

```go
type DecompositionResult struct {
    Features     []*feature.Feature
    Dependencies map[string][]string   // Feature ID → dependency IDs
}
```

### CLI

```bash
devteam intake --type external --file path/to/prd.md
```

### Behavior

1. PM reads the external specification document
2. PM identifies gaps, contradictions, and implicit assumptions
3. PM decomposes into N feature specs with dependency edges
4. Each feature gets its own `spec.md`, `acceptance.md`, `repos.yaml`
5. Dependencies between features are explicit (spec N depends on spec M)

### Validation

- File must exist and be readable
- Document must contain identifiable requirements