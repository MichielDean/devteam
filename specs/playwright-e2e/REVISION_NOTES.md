# Revision Required: review → construction

The review phase found issues that need to be fixed.

## Issues Found

F-001 CRITICAL: All 11 mandated Go integration tests missing. internal/api/server_test.go never modified on this branch (git diff main...HEAD empty, rg -ni health returns 0 matches). Implementation code (server.go:190,910-924 + pipeline.go:44 + ui/e2e/health.spec.ts) correct and minimal, but AC-001..AC-011 unverifiable. tasks.md T002 and T004 mandate 11 TestHealth* functions using httptest.NewServer: TestHealthGETReturns200AndBody (AC-001, byte-exact body via strings.TrimSpace), TestHealthGETNoRequestBody (AC-002), TestHealthVersionFromConfig (AC-003, Config{Version:9.9.9-test}), TestHealthGETIgnoresQueryParams (AC-004), TestHealthPanicReturns500 (AC-005), TestHealthPOSTReturns405 (AC-006), TestHealthPUTReturns405 (AC-007), TestHealthDELETEReturns405 (AC-008), TestHealthPATCHReturns405 (AC-009), TestHealthGETStill200After405s (AC-010), TestHealthTrailingSlash404 (AC-011). Add setupHealthTestServer(t, version) helper. See specs/playwright-e2e/review-report.md F-001 and tasks.md T002/T004.

## Instructions

Address ALL issues above before proceeding with your normal construction work.
Do NOT skip or ignore any of these issues.
