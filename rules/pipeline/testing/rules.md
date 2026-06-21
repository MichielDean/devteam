# Testing Phase Rules

## Purpose

Verify that what was built actually works in a running system. Not just that code compiles or unit tests pass.

Your defining question: **"Is this test real enough?"**

## Step 1: Spec-Implementation Drift Verification

Before writing any tests, compare the spec against what was built.

Read spec.md and acceptance.md, then compare with the implementation:

1. Did the spec ask for UI interactions? → Are there E2E tests?
2. Did the spec ask for error handling? → Are there tests for error paths?
3. Did the spec ask for real-time updates? → Are there SSE/WebSocket tests?
4. Frontend-backend contract: Does the frontend handle all error responses the backend can produce?
5. Are there acceptance criteria in acceptance.md that have NO corresponding implementation?

Document any drift. If the implementation doesn't match the spec, that's a finding — not necessarily a bug, but it needs to be checked.

## Step 2: Determine Testing Levels

### Level 1: Smoke Tests (ALWAYS REQUIRED)
Start the service. Hit every endpoint. Verify no panics, no crashes, no nil pointers.

### Level 2: Integration Tests (REQUIRED FOR API CHANGES)
Full request/response cycles through real HTTP endpoints with real middleware.

### Level 3: E2E Tests (REQUIRED FOR UI CHANGES)
Load the web UI in a browser. Click through workflows. Verify no console errors.

### Level 4: Unit Tests (AS APPROPRIATE)
Business logic in isolation. State machine transitions. Serialization.

### Test Selection Matrix

| What changed | Smoke | Integration | E2E | Unit |
|---|---|---|---|---|
| HTTP API handlers | **YES** | **YES** | — | YES |
| Frontend/UI components | **YES** | **YES** | **YES** | YES |
| State machine logic | YES | — | — | **YES** |
| Gate evaluator | YES | — | — | **YES** |
| CLI commands | **YES** | — | — | YES |
| Middleware/auth | **YES** | **YES** | — | YES |
| Database operations | **YES** | **YES** | — | YES |

## Step 3: Write and Execute Smoke Tests

### Smoke Test Requirements

Every feature MUST have smoke tests that verify:

1. **Service starts**: Build the binary and start it. Verify no panics.
2. **Every endpoint responds**: Hit each endpoint. Verify expected status codes.
3. **No nil pointer panics**: Hit each endpoint. Verify the server doesn't crash.
4. **Empty state works**: GET endpoints return `200 []` or `200 {}`, not `null`.
5. **Recovery middleware works**: Send malformed requests. Verify 500 errors are caught, not panics.

### Smoke Test Template

```go
func TestSmokeServerStartsAndResponds(t *testing.T) {
    srv := NewTestServer(t)
    defer srv.Close()

    resp, err := http.Get(srv.URL + "/api/features")
    if err != nil {
        t.Fatalf("GET /api/features: %v", err)
    }
    if resp.StatusCode != http.StatusOK {
        t.Errorf("GET /api/features: got %d, want %d", resp.StatusCode, http.StatusOK)
    }
    // Verify body is [] not null
    body, _ := io.ReadAll(resp.Body)
    if string(body) == "null" {
        t.Error("GET /api/features: got null, want []")
    }
}
```

### Smoke Test Checklist

- [ ] Server starts without panic
- [ ] Every endpoint returns expected status code
- [ ] Every endpoint returns valid JSON (not HTML error pages)
- [ ] Recovery middleware catches panics (returns 500, not connection drop)
- [ ] Empty collections return `[]` not `null`
- [ ] Invalid routes return 404
- [ ] Malformed JSON returns 400

## Step 4: Write and Execute Integration Tests

### Integration Test Requirements

For every API endpoint, test:

1. **Happy path**: Valid input → expected success response
2. **Missing required fields**: Omit required fields → 400
3. **Invalid input types**: Wrong types → 400
4. **Not found**: Missing resources → 404
5. **Conflict**: Duplicate creation → 409
6. **Full response shape**: Verify every field in the response matches the contract

### Integration Test Template

```go
func TestIntegrationCreateAndGetFeature(t *testing.T) {
    srv := NewTestServer(t)
    defer srv.Close()

    // Create
    body := `{"title": "Test Feature", "priority": "P1"}`
    resp, err := http.Post(srv.URL+"/api/features", "application/json", strings.NewReader(body))
    if err != nil {
        t.Fatalf("POST /api/features: %v", err)
    }
    if resp.StatusCode != http.StatusCreated {
        t.Errorf("POST /api/features: got %d, want %d", resp.StatusCode, http.StatusCreated)
    }

    // Get
    resp, err = http.Get(srv.URL + "/api/features")
    if err != nil {
        t.Fatalf("GET /api/features: %v", err)
    }
    // Verify response shape matches contract
    var features []Feature
    if err := json.NewDecoder(resp.Body).Decode(&features); err != nil {
        t.Fatalf("Decode response: %v", err)
    }
    if len(features) != 1 {
        t.Errorf("Expected 1 feature, got %d", len(features))
    }
}
```

### Error Path Testing

For every endpoint, specifically test:
- **400 Bad Request**: Missing required fields, invalid types, out-of-range values
- **404 Not Found**: Requesting non-existent resources
- **409 Conflict**: Creating duplicate resources
- **500 Internal Server Error**: Should be caught by recovery middleware, not panic

### JSON Shape Verification

Every integration test must verify that:
- Response is valid JSON
- Collections are `[]` not `null`
- Error responses have `{"error": "code", "details": "message"}` structure
- No unexpected null fields in success responses

## Step 5: Write and Execute E2E Tests (If UI Changed)

### E2E Test Requirements

If the feature includes a UI:

1. **Page loads**: Open the page, verify no console errors
2. **Data renders**: Verify that data from the API appears in the UI
3. **Interactions work**: Click buttons, fill forms, verify responses
4. **Error states display**: Trigger errors, verify error messages appear
5. **Empty state displays**: When no data exists, verify empty state message

### E2E Test Framework

Use Playwright (or equivalent) for browser automation:
```typescript
test('feature list loads and displays features', async ({ page }) => {
    await page.goto('/features');
    await expect(page.locator('[data-testid="feature-list"]')).toBeVisible();
    const errors = await page.consoleErrors();
    expect(errors).toHaveLength(0);
});
```

### data-testid Requirements

All interactive UI elements must have `data-testid` attributes:
- Buttons: `data-testid="create-feature-button"`
- Forms: `data-testid="create-feature-form"`
- Lists: `data-testid="feature-list"`
- Items: `data-testid="feature-item-{id}"`

## Step 6: Write and Execute Unit Tests

### Unit Test Requirements

Test business logic in isolation:

1. **State machine transitions**: For every entity with state, test all valid transitions and verify invalid transitions are rejected
2. **Serialization**: Verify JSON marshal/unmarshal for all API types, especially empty collections
3. **Validation**: Test input validation for all fields (required, type, length, format)
4. **Business rules**: Test specific business logic (calculations, filters, transformations)

### Unit Test Template

```go
func TestFeatureStateTransitions(t *testing.T) {
    tests := []struct {
        name    string
        from    Phase
        to      Phase
        wantErr bool
    }{
        {"draft to inception", PhaseDraft, PhaseInception, false},
        {"inception to planning", PhaseInception, PhasePlanning, false},
        {"draft to planning (skip)", PhaseDraft, PhasePlanning, true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            f := NewFeature()
            f.Current = tt.from
            err := f.AdvanceTo(tt.to)
            if (err != nil) != tt.wantErr {
                t.Errorf("AdvanceTo(%s → %s): error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
            }
        })
    }
}
```

## Step 7: Agent Failure Mode Verification

When testing agent-generated code, specifically verify:

1. **Nil pointer chains**: Start the service, hit every endpoint, verify no panics
2. **Null arrays**: Verify every collection field returns [] not null when empty
3. **Phantom methods**: Verify the code compiles AND runs (methods exist, types match)
4. **Over-engineering**: Check line counts. If the API server is 3x the test suite, something's wrong
5. **Missing error paths**: Test 404, 400, 409, empty state, malformed input

## Step 8: Proof of Work

Name specific files, methods, and assertions. "Tests pass" is not evidence.

Your test report MUST include:

1. **Smoke tests**: "I started the server on :8765 and hit every endpoint" — list the endpoints and status codes
2. **Integration tests**: "I created a feature, retrieved it, verified all 6 phase states" — list the scenarios
3. **E2E tests**: "I loaded the UI in Playwright, verified no console errors" — list the pages and interactions
4. **Null/empty checks**: "I verified artifacts, checks, dependencies, repos all return [] not null" — list the fields
5. **State machine transitions**: "I tested start, advance, recirculate, cancel" — list the transitions tested

## Step 9: Anti-Fake-Report

An agent can write "all 56 tests pass" in a markdown file without running any tests. Your test report MUST include:
- Exact commands to reproduce each test
- Exact assertions verified
- Exact endpoints hit during smoke testing
- Console output or screenshots from E2E tests

A test report that says "all tests pass" without reproducible commands is not a test report — it's a claim.

## Quality Gate

Testing is complete when:
1. Smoke tests pass: service starts, every endpoint returns expected status codes
2. Integration tests pass: full HTTP cycles work, JSON shapes match contract ([] not null)
3. E2E tests pass (if UI changed): frontend loads, renders data, no console errors
4. State machine verified: all valid transitions work, invalid transitions rejected
5. Spec drift checked: every user story in spec has a corresponding test
6. Every acceptance criterion has at least one test
7. No nil pointer panics, no null-vs-empty-array mismatches, no untested error paths

## Findings Have No Severity Tiers

Every finding is either "needs fixing" (recirculate) or "doesn't need fixing" (don't mention it).

**ANY failing test is an automatic recirculate.** A codebase with red tests is broken, period.
**ANY nil pointer panic is an automatic recirculate.** If the server crashes, it's not ready.
**ANY null-vs-empty-array mismatch is a finding.** Arrays in JSON must be [], not null.