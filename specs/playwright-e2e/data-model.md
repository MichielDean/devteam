# Data Model: playwright-e2e (GET /api/health)

## Entities

### HealthStatus
Ephemeral response entity. **Not persisted.** No DB table, no file, no lifecycle.

- **Attributes**:
  - `status`: `string`, required, fixed value `"ok"`, no validation (constant)
  - `version`: `string`, required, sourced from `config.Config.Version` at request time, no validation (reflects loaded config faithfully, including empty string)
- **Relationships**: none
- **Constraints**: none (not stored)
- **State Transitions**: none (stateless response, recomputed per request)
- **JSON shape**: `{"status":"ok","version":"<version>"}` — field order: `status` then `version` (matches spec/input.md byte-exact expectation for CON-006; Go struct field order determines JSON key order)

### Go struct (to be defined in `internal/api/server.go`)
```go
type healthResponse struct {
    Status  string `json:"status"`
    Version string `json:"version"`
}
```
`json.Encoder.Encode` emits fields in struct declaration order → `status` before `version` → byte-exact `{"status":"ok","version":"1.0"}` (CON-006).

## Data Integrity Rules
- No persistence → no integrity rules.
- `version` field is read-only from config; handler does not mutate.
- Empty `config.Version` → empty `version` field (per spec assumption, faithful reflection).

## API Contracts
See `contracts/GET-api-health.md`.