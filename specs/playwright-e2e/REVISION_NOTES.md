# Revision Required: review → construction

The review phase found issues that need to be fixed.

## Issues Found

F-001 BLOCKING: Go integration tests for AC-001..AC-011 missing. internal/api/server_test.go unchanged by feature branch. Implement tasks.md T002 (5 tests: TestHealthGETReturns200AndBody, TestHealthGETNoRequestBody, TestHealthVersionFromConfig, TestHealthGETIgnoresQueryParams, TestHealthPanicReturns500) and T004 (6 tests: TestHealthPOSTReturns405, TestHealthPUTReturns405, TestHealthDELETEReturns405, TestHealthPATCHReturns405, TestHealthGETStill200After405s, TestHealthTrailingSlash404) plus setupHealthTestServer helper using httptest.NewServer pattern. Implementation code (healthHandler, route, Pipeline.Config(), Playwright spec) is correct and minimal — only the Go test artifact is missing. See specs/playwright-e2e/review-report.md.

## Instructions

Address ALL issues above before proceeding with your normal construction work.
Do NOT skip or ignore any of these issues.
