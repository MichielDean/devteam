# Security Extension (Mandatory for P1 Features)

When this extension is loaded (automatically for priority-1 features), add these checks to every phase:

## Inception (PM)
- Identify security-sensitive user stories (authentication, authorization, data access)
- Add security-specific acceptance criteria: "Given an unauthenticated user, when they access [endpoint], then they receive 401/403"

## Planning (Architect)
- Document authentication and authorization approach
- Identify sensitive data flows (PII, credentials, tokens)
- Specify input validation rules (length limits, character whitelisting, type checking)
- Add security checkpoints to done conditions

## Construction (Developer)
- Never log secrets, tokens, or PII
- Use constant-time comparison for sensitive values
- Validate all input at the boundary (HTTP handlers, not internal functions)
- Set security headers: Content-Security-Policy, X-Content-Type-Options, X-Frame-Options
- Rate-limit sensitive endpoints

## Review (Reviewer)
- Verify authentication is required on protected endpoints
- Verify authorization checks are role-based, not just authenticated
- Verify input validation on all user-facing endpoints
- Verify no secrets in logs, error messages, or responses
- Verify CORS is restrictive (not Access-Control-Allow-Origin: *)
- Verify rate limiting on sensitive endpoints

## Testing (Tester)
- Test authentication enforcement: unauthenticated requests return 401
- Test authorization enforcement: unauthorized requests return 403
- Test input validation: malformed input returns 400, not 500
- Test rate limiting: excessive requests are throttled
- Test security headers are present