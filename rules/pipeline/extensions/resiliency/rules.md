# Resiliency Extension (Mandatory for P1 Features)

When this extension is loaded (automatically for priority-1 features), add these checks to every phase:

## Inception (PM)
- Identify resilience requirements: retry policies, timeout limits, fallback behaviors
- Add resilience acceptance criteria: "Given a downstream service timeout, when the request takes >5s, then the system returns a timeout error (not a 500)"

## Planning (Architect)
- Document retry policies: which operations retry, how many times, backoff strategy
- Document timeout limits: per-endpoint and global
- Document circuit breaker behavior: when to open, when to close, fallback behavior
- Document graceful degradation: what functionality is preserved when dependencies fail

## Construction (Developer)
- Use context.WithTimeout for all external calls (max 30 seconds)
- Implement retry with exponential backoff for transient failures
- Return meaningful error codes, not generic 500s
- Don't hang on unavailable services — timeout and degrade gracefully
- Log errors with context (entity, operation), not just "error"

## Review (Reviewer)
- Verify all external calls have timeouts
- Verify error messages include domain context (entity, operation)
- Verify no errors are silently swallowed
- Verify errors use fmt.Errorf("pkg: context: %w", err) wrapping
- Verify no fmt.Fprintf(os.Stderr) for errors — use structured logging

## Testing (Tester)
- Test timeout behavior: what happens when external calls take too long
- Test retry behavior: what happens when external calls fail transiently
- Test concurrent access: what happens when multiple requests hit the same resource
- Test resource limits: what happens when memory/CPU/connections are exhausted