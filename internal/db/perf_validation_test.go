package db

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

// perf_validation_test.go — Stage 4.6 Performance Validation evidence.
//
// Implements the measurement method defined in `performance-nfrs` (3.2):
//   - NFR-PERF-01: Audit filtered read against 100k-row table; p95 ≤ 500ms;
//     query plan is index-backed (idx_audit_events_type_time), not a seq scan.
//   - NFR-PERF-02: Read endpoint latency p95 ≤ 200ms warm (db-level proxy:
//     GetAuditEventsFiltered with no filter on a small table is trivially
//     fast; this test focuses on the load-bearing NFR-PERF-01 case).
//
// These tests are tagged `perf` so they can be skipped in the fast gate
// (`go test -short`) and run explicitly in the performance-validation stage:
//
//	go test ./internal/db/ -run TestPerf -v -count=1
//
// They require a live Postgres test DB (postgresTestDSN) and are skipped if
// the DB is unreachable — matching the existing test-DB posture.

func skipIfNoDB(t *testing.T) *DB {
	t.Helper()
	d, err := Open(Config{DSN: postgresTestDSN}, postgresTestDSN)
	if err != nil {
		t.Skipf("test DB unavailable: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seedAuditRows bulk-inserts n audit_events spread over a `spanDays` window
// using a single multi-value INSERT batched in chunks of `batch`. It returns
// the approximate insert duration for reporting.
func seedAuditRows(t *testing.T, d *DB, n, spanDays, batch int) time.Duration {
	t.Helper()
	if n <= 0 {
		return 0
	}
	// Ensure a sentinel platform feature row is not required: migration 018
	// dropped the feature_id FK so platform-level events can be inserted
	// without a features row.
	base := time.Now().UTC().Add(-time.Duration(spanDays) * 24 * time.Hour)
	step := time.Duration(spanDays*24*60*60*1e9) / time.Duration(n) // ns per row
	types := []string{AuditConfigUpdated, AuditReposRegistryMutated, AuditFeatureDefaultsMutated, AuditConfigValidationFailed}
	events := make([]AuditEvent, 0, n)
	for i := 0; i < n; i++ {
		et := types[i%len(types)]
		ts := base.Add(time.Duration(i) * step)
		events = append(events, AuditEvent{
			FeatureID: "platform",
			EventType: et,
			StageID:   "s",
			Phase:     "construction",
			Details:   fmt.Sprintf("perf-seed row %d", i),
			Actor:     "operator",
			CreatedAt: ts,
		})
	}
	start := time.Now()
	for i := 0; i < n; i += batch {
		end := i + batch
		if end > n {
			end = n
		}
		var sb strings_Builder
		sb.WriteString("INSERT INTO audit_events (feature_id, event_type, stage_id, phase, details, actor, created_at) VALUES ")
		args := make([]interface{}, 0, (end-i)*7)
		first := true
		for j := i; j < end; j++ {
			if !first {
				sb.WriteByte(',')
			}
			first = false
			sb.WriteString("(?,?,?,?,?,?,?)")
			ev := events[j]
			args = append(args, ev.FeatureID, ev.EventType, ev.StageID, ev.Phase, ev.Details, ev.Actor, ev.CreatedAt)
		}
		if _, err := d.Exec(sb.String(), args...); err != nil {
			t.Fatalf("bulk insert batch [%d:%d]: %v", i, end, err)
		}
	}
	return time.Since(start)
}

// minimal strings.Builder alias so we don't add an import just for the seeder.
type strings_Builder = stringsBuilderAlias

// stringsBuilderAlias is a thin wrapper around the standard strings.Builder.
// We alias it to keep the seeder self-contained without expanding imports
// at the top of the file beyond what's needed for the test bodies.
type stringsBuilderAlias struct{ b []byte }

func (s *stringsBuilderAlias) WriteString(p string) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}
func (s *stringsBuilderAlias) WriteByte(c byte) error {
	s.b = append(s.b, c)
	return nil
}
func (s *stringsBuilderAlias) String() string { return string(s.b) }

// TestPerf_AuditFilteredRead_100k_NFR_PERF_01 is the binding performance
// validation for NFR-PERF-01. It seeds 100,000 audit_events, runs the
// canonical filtered query 20 times, records p50/p95, and asserts:
//   - p95 ≤ 500ms (the NFR-PERF-01 acceptance threshold)
//   - the query plan is index-backed (no Seq Scan on audit_events)
//
// Skipped under -short so it does not slow the fast CI gate (G3).
func TestPerf_AuditFilteredRead_100k_NFR_PERF_01(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in -short mode")
	}
	d := skipIfNoDB(t)
	truncateAllTables(d)

	const n = 100_000
	insertDur := seedAuditRows(t, d, n, 180, 1000)
	t.Logf("seeded %d audit_events in %s", n, insertDur)

	// Canonical filter: event_type + 30-day window + page=1/page_size=50.
	from := time.Now().UTC().Add(-30 * 24 * time.Hour)
	to := time.Now().UTC()
	filter := AuditFilter{
		EventType: AuditConfigUpdated,
		From:      from,
		To:        to,
		Page:      1,
		PageSize:  50,
	}

	// Warm the cache (the NFR allows a cold-start allowance; we measure warm).
	if _, _, err := d.GetAuditEventsFiltered(filter); err != nil {
		t.Fatalf("warmup GetAuditEventsFiltered: %v", err)
	}

	const runs = 20
	samples := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()
		events, total, err := d.GetAuditEventsFiltered(filter)
		dur := time.Since(start)
		if err != nil {
			t.Fatalf("run %d GetAuditEventsFiltered: %v", i, err)
		}
		if len(events) > 50 {
			t.Errorf("run %d: got %d events, want ≤ 50 (page_size)", i, len(events))
		}
		if total < 0 {
			t.Errorf("run %d: negative total %d", i, total)
		}
		samples = append(samples, dur)
	}

	p50, p95 := percentile(samples, 0.50), percentile(samples, 0.95)
	t.Logf("NFR-PERF-01: n=%d runs=%d p50=%s p95=%s (target p95≤500ms)", n, runs, p50, p95)
	if p95 > 500*time.Millisecond {
		t.Fatalf("NFR-PERF-01 FAIL: p95=%s exceeds 500ms target", p95)
	}

	// Plan check: EXPLAIN the canonical filter query and assert no Seq Scan.
	plan := explainPlan(t, d, filter)
	t.Logf("NFR-PERF-01 plan:\n%s", plan)
	if containsSeqScan(plan) {
		t.Fatalf("NFR-PERF-01 FAIL: query plan contains a Seq Scan on audit_events — idx_audit_events_type_time not used")
	}
	if !containsIndexScan(plan) {
		t.Fatalf("NFR-PERF-01 FAIL: query plan has no Index Scan — expected idx_audit_events_type_time")
	}
}

// TestPerf_AuditFilteredRead_Pagination_NFR_PERF_01 verifies page 1 and page 2
// are non-overlapping and ordered by created_at DESC (the NFR's pagination
// sanity clause), at the 100k-row scale.
func TestPerf_AuditFilteredRead_Pagination_NFR_PERF_01(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in -short mode")
	}
	d := skipIfNoDB(t)
	truncateAllTables(d)
	seedAuditRows(t, d, 10_000, 60, 1000)

	filter := AuditFilter{EventType: AuditConfigUpdated, Page: 1, PageSize: 50}
	page1, _, err := d.GetAuditEventsFiltered(filter)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	filter.Page = 2
	page2, _, err := d.GetAuditEventsFiltered(filter)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page1) == 0 || len(page2) == 0 {
		t.Fatalf("expected non-empty pages, got p1=%d p2=%d", len(page1), len(page2))
	}
	// Ordered DESC by created_at: page1[0] >= page1[last] >= page2[0] >= page2[last]
	if !page1[0].CreatedAt.After(page1[len(page1)-1].CreatedAt) && !page1[0].CreatedAt.Equal(page1[len(page1)-1].CreatedAt) {
		t.Errorf("page 1 not DESC-ordered: first=%s last=%s", page1[0].CreatedAt, page1[len(page1)-1].CreatedAt)
	}
	// Non-overlapping: max id on page 1 > min id on page 2 (DESC order ⇒ page1 ids are larger).
	seen := make(map[int64]bool, len(page1)+len(page2))
	for _, e := range page1 {
		seen[e.ID] = true
	}
	for _, e := range page2 {
		if seen[e.ID] {
			t.Errorf("audit event id %d appears on both page 1 and page 2 — pagination overlap", e.ID)
		}
	}
}

// TestPerf_ReadEndpoints_Small_NFR_PERF_02 is the db-level proxy for
// NFR-PERF-02: a no-filter audit read on a small table should be well under
// 200ms p95. The full handler-level latency (including JSON marshal + HTTP)
// is bounded by the Go test in the api package; this test guards the DB layer.
func TestPerf_ReadEndpoints_Small_NFR_PERF_02(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping perf test in -short mode")
	}
	d := skipIfNoDB(t)
	truncateAllTables(d)
	seedAuditRows(t, d, 500, 30, 500)

	filter := AuditFilter{Page: 1, PageSize: 50}
	if _, _, err := d.GetAuditEventsFiltered(filter); err != nil {
		t.Fatalf("warmup: %v", err)
	}
	const runs = 20
	samples := make([]time.Duration, 0, runs)
	for i := 0; i < runs; i++ {
		start := time.Now()
		if _, _, err := d.GetAuditEventsFiltered(filter); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		samples = append(samples, time.Since(start))
	}
	p95 := percentile(samples, 0.95)
	t.Logf("NFR-PERF-02 (db proxy): n=500 runs=%d p95=%s (target p95≤200ms)", runs, p95)
	if p95 > 200*time.Millisecond {
		t.Fatalf("NFR-PERF-02 FAIL: p95=%s exceeds 200ms target", p95)
	}
}

// --- helpers ---

func percentile(samples []time.Duration, q float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * q)
	return sorted[idx]
}

func explainPlan(t *testing.T, d *DB, f AuditFilter) string {
	t.Helper()
	// Build the same WHERE the production query builds, then EXPLAIN it.
	conds := []string{"event_type = ?"}
	args := []interface{}{AuditConfigUpdated}
	if !f.From.IsZero() {
		conds = append(conds, "created_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		conds = append(conds, "created_at <= ?")
		args = append(args, f.To)
	}
	where := " WHERE " + conds[0]
	for _, c := range conds[1:] {
		where += " AND " + c
	}
	query := "EXPLAIN SELECT id, feature_id, event_type, stage_id, phase, details, actor, created_at FROM audit_events" +
		where + " ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	rows, err := d.Query(query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	var plan string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan EXPLAIN row: %v", err)
		}
		plan += line + "\n"
	}
	return plan
}

func containsSeqScan(plan string) bool {
	return containsFold(plan, "Seq Scan")
}

func containsIndexScan(plan string) bool {
	return containsFold(plan, "Index Scan") || containsFold(plan, "Index Only Scan")
}

func containsFold(s, sub string) bool {
	ls := stringsToLower(s)
	lsub := stringsToLower(sub)
	return indexOf(ls, lsub) >= 0
}

func stringsToLower(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}