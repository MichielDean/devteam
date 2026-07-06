package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/stage"
)

// BoltResult holds the outcome of running one Bolt through Construction stages.
type BoltResult struct {
	BoltNumber        int
	IsWalkingSkeleton bool
	StageResults      []*StageRunResult
	Failed            bool
	FailureStage      string
	FailureReason     string
	Duration          time.Duration
}

// AutonomyMode constants for construction.
const (
	AutonomyGated      = "gated"      // gate every Bolt
	AutonomyAutonomous = "autonomous" // skip per-Bolt gates, halt on failure
)

// Execution modes (apply to all phases, not just construction).
const (
	ExecutionHuman      = "human"      // Mode 1: every stage started and approved manually
	ExecutionGuided     = "guided"     // Mode 2: auto-run stages, pause at phase-end review gates
	ExecutionAutonomous = "autonomous" // Mode 3: auto-run everything, auto-approve all gates, LLM answers questions
)

// PrepareBolts reads units-of-work and the dependency DAG from inception output
// (stages 2.7 and 2.8) and creates Bolt records in the bolts table.
// Bolt 1 is the walking skeleton (no dependencies). Remaining bolts carry their
// dependencies so RunAllBolts can batch independent ones for parallel dispatch.
func (p *Pipeline) PrepareBolts(f *feature.Feature) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	// Check if bolts already prepared
	existing, _ := p.database.GetBolts(f.ID)
	if len(existing) > 0 {
		log.Printf("PrepareBolts: %s already has %d bolts", f.ID, len(existing))
		return nil
	}

	// Read units-of-work from artifacts
	// The architect produces unit-of-work, dependency-dag, story-map in stage 2.7
	units := p.extractUnitsFromArtifacts(f)
	if len(units) == 0 {
		log.Printf("PrepareBolts: no units found for %s — creating single Bolt", f.ID)
		units = []string{"unit-1"}
	}

	// Build bolt→units and bolt→depends-on mapping from the DAG artifact.
	// Bolt numbers are 1-indexed in declaration order; the first bolt is the
	// walking skeleton (smallest end-to-end slice, no dependencies).
	boltUnits := p.groupUnitsIntoBolts(units)
	boltDeps := p.extractBoltDependencies(f, len(boltUnits))

	for i, bu := range boltUnits {
		boltNum := i + 1
		isWalkingSkeleton := i == 0
		deps := boltDeps[boltNum]
		if isWalkingSkeleton {
			deps = nil
		}
		if err := p.database.CreateBolt(f.ID, boltNum, bu, deps, isWalkingSkeleton); err != nil {
			return fmt.Errorf("creating bolt %d: %w", boltNum, err)
		}
	}

	log.Printf("PrepareBolts: created %d bolts for %s (bolt 1 = walking skeleton)", len(boltUnits), f.ID)
	return nil
}

// extractUnitsFromArtifacts reads the units-of-work from stage 2.7 artifacts.
// ponytail: reads from spec artifacts DB; if no units found, returns empty.
func (p *Pipeline) extractUnitsFromArtifacts(f *feature.Feature) []string {
	// Try reading unit-of-work artifact
	content, err := p.specProvider.ReadArtifact(f.ID, "unit-of-work")
	if err != nil || content == "" {
		return nil
	}

	// Parse units from content — look for unit identifiers
	var units []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## Unit ") || strings.HasPrefix(line, "### Unit ") {
			// Extract unit ID from heading
			parts := strings.SplitN(line, " ", 4)
			if len(parts) >= 3 {
				units = append(units, parts[2])
			}
		}
	}
	return units
}

// groupUnitsIntoBolts bundles units into Bolts.
// ponytail: one unit per Bolt for now. The dependency DAG (not the grouping)
// is what drives parallel batching in RunAllBolts.
func (p *Pipeline) groupUnitsIntoBolts(units []string) [][]string {
	bolts := make([][]string, len(units))
	for i, u := range units {
		bolts[i] = []string{u}
	}
	return bolts
}

// extractBoltDependencies reads the dependency-dag artifact from stage 2.7 and
// returns a map of bolt number → bolt numbers it depends on.
// The DAG lists unit→unit dependencies; since one unit maps to one bolt, the
// translation is direct. Missing or unparseable DAG → no dependencies (bolts
// run in declaration order, which is still correct, just not parallel).
func (p *Pipeline) extractBoltDependencies(f *feature.Feature, boltCount int) map[int][]int {
	deps := make(map[int][]int, boltCount)
	content, err := p.specProvider.ReadArtifact(f.ID, "dependency-dag")
	if err != nil || content == "" {
		return deps
	}

	// Build unit→bolt-number lookup from the same ordering groupUnitsIntoBolts used.
	// We re-derive units here to keep the mapping stable if grouping changes later.
	units := p.extractUnitsFromArtifacts(f)
	if len(units) == 0 {
		return deps
	}
	unitToBolt := make(map[string]int, len(units))
	for i, u := range units {
		unitToBolt[u] = i + 1
	}

	// Parse lines like "- unit-2 depends on unit-1" or "unit-2 -> unit-1".
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip leading bullets/arrows
		line = strings.TrimPrefix(line, "-")
		line = strings.TrimSpace(line)

		var dependent, dependency string
		switch {
		case strings.Contains(line, " depends on "):
			parts := strings.SplitN(line, " depends on ", 2)
			if len(parts) != 2 {
				continue
			}
			dependent, dependency = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		case strings.Contains(line, " -> "):
			parts := strings.SplitN(line, " -> ", 2)
			if len(parts) != 2 {
				continue
			}
			dependent, dependency = strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		default:
			continue
		}

		depBolt, ok1 := unitToBolt[dependent]
		prereqBolt, ok2 := unitToBolt[dependency]
		if !ok1 || !ok2 || depBolt == prereqBolt {
			continue
		}
		deps[depBolt] = append(deps[depBolt], prereqBolt)
	}

	return deps
}

// RunBolt runs stages 3.1-3.5 for one Bolt (serialized, one tmux session).
// In gated mode, opens a gate after the Bolt. In autonomous mode, skips per-Bolt gates.
// Failures always halt (retry/skip/abort via halt-and-ask).
func (p *Pipeline) RunBolt(ctx context.Context, f *feature.Feature, boltNumber int, onOutput OutputLineCallback) (*BoltResult, error) {
	if p.database == nil {
		return nil, fmt.Errorf("database required")
	}

	bolts, err := p.database.GetBolts(f.ID)
	if err != nil {
		return nil, fmt.Errorf("getting bolts: %w", err)
	}

	var bolt *db.BoltRow
	for _, b := range bolts {
		if b.BoltNumber == boltNumber {
			bolt = &b
			break
		}
	}
	if bolt == nil {
		return nil, fmt.Errorf("bolt %d not found for feature %s", boltNumber, f.ID)
	}

	// Ensure per-Bolt stage rows exist for this bolt.
	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}
	if err := p.database.InitBoltStages(f.ID, boltNumber, scope); err != nil {
		return nil, fmt.Errorf("init bolt stages for bolt %d: %w", boltNumber, err)
	}

	// Set CurrentBolt so RunStage resolves the per-Bolt stage row and session.
	f.CurrentBolt = boltNumber
	p.saveFeatureState(f)

	now := time.Now()
	p.database.UpdateBoltStatus(f.ID, boltNumber, "in_progress")
	p.database.RecordAuditEvent(f.ID, db.AuditBoltStarted, "", stage.PhaseConstruction,
		fmt.Sprintf("bolt %d (walking skeleton=%v)", boltNumber, bolt.IsWalkingSkeleton))

	constructionStages := []string{"3.1", "3.2", "3.3", "3.4", "3.5"}
	var stageResults []*StageRunResult

	result := &BoltResult{
		BoltNumber:        boltNumber,
		IsWalkingSkeleton: bolt.IsWalkingSkeleton,
		Duration:          0,
	}

	for _, stageID := range constructionStages {
		// Check if this per-Bolt stage is already completed for this bolt — skip it
		fs, _ := p.database.GetFeatureStageForBolt(f.ID, stageID, boltNumber)
		if fs != nil && fs.Status == stage.StatusCompleted {
			log.Printf("RunBolt: skipping stage %s bolt %d — already completed", stageID, boltNumber)
			continue
		}

		// Check if this stage applies to the feature's scope
		stageDef, err := p.database.GetStageDefinition(stageID)
		if err != nil {
			log.Printf("RunBolt: skipping stage %s — definition not found: %v", stageID, err)
			continue
		}

		scopeMatch := false
		for _, s := range stageDef.Scopes {
			if s == scope {
				scopeMatch = true
				break
			}
		}
		if !scopeMatch {
			log.Printf("RunBolt: skipping stage %s — not in scope %s", stageID, scope)
			continue
		}

		log.Printf("RunBolt: running stage %s for bolt %d", stageID, boltNumber)
		stageResult, err := p.RunStage(ctx, f, stageID, onOutput)
		if err != nil {
			result.Failed = true
			result.FailureStage = stageID
			result.FailureReason = err.Error()
			p.database.RecordAuditEvent(f.ID, db.AuditBoltFailed, "", stage.PhaseConstruction,
				fmt.Sprintf("bolt %d failed at stage %s: %v", boltNumber, stageID, err))
			p.database.UpdateBoltStatus(f.ID, boltNumber, "failed")
			return result, nil
		}
		stageResults = append(stageResults, stageResult)

		// If gate is open, auto-approve in autonomous/guided mode; pause in human mode.
		if stageResult.Gate != nil && stageResult.Gate.IsOpen() {
			mode := f.ExecutionMode
			if mode == "" {
				mode = ExecutionHuman
			}
			if mode == ExecutionAutonomous || mode == ExecutionGuided {
				// Auto-approve per-bolt stages — the batch-level gate (if any)
				// is handled by presentBatchGate in RunAllBolts.
				log.Printf("RunBolt: auto-approving stage %s bolt %d (%s mode)", stageID, boltNumber, mode)
				if _, err := p.ApproveStage(f, stageID, boltNumber); err != nil {
					log.Printf("RunBolt: auto-approve failed for %s bolt %d: %v — pausing", stageID, boltNumber, err)
					p.database.UpdateBoltStatus(f.ID, boltNumber, "pending")
					result.StageResults = stageResults
					result.Duration = time.Since(now)
					return result, nil
				}
				// Gate approved — continue to next stage in the bolt
				continue
			}
			// Human mode — pause for manual approval
			log.Printf("RunBolt: stage %s opened gate — pausing bolt %d (human mode)", stageID, boltNumber)
			p.database.UpdateBoltStatus(f.ID, boltNumber, "pending")
			result.StageResults = stageResults
			result.Duration = time.Since(now)
			return result, nil
		}

		// If stage was rejected (smoke/reviewer), halt
		if stageResult.OutcomeSource == "smoke_failed" || stageResult.OutcomeSource == "reviewer_rejected" {
			result.Failed = true
			result.FailureStage = stageID
			result.FailureReason = strings.Join(stageResult.SmokeFailures, "; ")
			if stageResult.ReviewerResult != nil {
				result.FailureReason = stageResult.ReviewerResult.Notes
			}
			p.database.RecordAuditEvent(f.ID, db.AuditHaltAndAsk, stageID, stage.PhaseConstruction,
				fmt.Sprintf("bolt %d halted at stage %s: %s", boltNumber, stageID, result.FailureReason))
			return result, nil
		}
	}

	result.StageResults = stageResults
	result.Duration = time.Since(now)
	// If we got through all stages without pausing or failing, bolt is complete
	p.database.UpdateBoltStatus(f.ID, boltNumber, "completed")
	p.database.RecordAuditEvent(f.ID, db.AuditBoltCompleted, "", stage.PhaseConstruction,
		fmt.Sprintf("bolt %d completed all construction stages", boltNumber))

	return result, nil
}

// LadderPrompt fires once after the walking skeleton (Bolt 1). Asks the user
// to choose autonomy mode for remaining Bolts: gated or autonomous.
// Returns the chosen mode. Stored on feature.AutonomyMode.
func (p *Pipeline) LadderPrompt(f *feature.Feature, mode string) error {
	if mode != AutonomyGated && mode != AutonomyAutonomous {
		return fmt.Errorf("invalid autonomy mode %s — use 'gated' or 'autonomous'", mode)
	}

	f.AutonomyMode = mode
	p.saveFeatureState(f)

	if p.database != nil {
		p.database.RecordAuditEvent(f.ID, db.AuditLadderPrompt, "", stage.PhaseConstruction,
			fmt.Sprintf("autonomy mode set to %s", mode))
	}

	log.Printf("LadderPrompt: feature %s autonomy mode = %s", f.ID, mode)
	return nil
}

// RunConstructionStages runs stages 3.6 (build+test) and 3.7 (CI pipeline) once
// at the end across all Bolts.
func (p *Pipeline) RunConstructionStages(ctx context.Context, f *feature.Feature, onOutput OutputLineCallback) ([]*StageRunResult, error) {
	var results []*StageRunResult

	for _, stageID := range []string{"3.6", "3.7"} {
		stageDef, err := p.database.GetStageDefinition(stageID)
		if err != nil {
			log.Printf("RunConstructionStages: skipping %s — not found: %v", stageID, err)
			continue
		}

		scope := f.Scope
		if scope == "" {
			scope = stage.ScopeFeature
		}
		scopeMatch := false
		for _, s := range stageDef.Scopes {
			if s == scope {
				scopeMatch = true
				break
			}
		}
		if !scopeMatch {
			continue
		}

		result, err := p.RunStage(ctx, f, stageID, onOutput)
		if err != nil {
			return results, fmt.Errorf("stage %s: %w", stageID, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// RunAllBolts runs the full construction phase per the AIDLC v2 spec:
//
//  1. Bolt 1 (walking skeleton) runs first, always sequential, always gated.
//  2. The ladder prompt fires once. The caller (UI) sets f.AutonomyMode via
//     LadderPrompt; this method blocks until that happens.
//  3. Remaining bolts run in dependency-ordered batches. Bolts whose
//     dependencies are all completed run concurrently in one batch; a single
//     batch-level gate covers every bolt in it (skipped if autonomous).
//  4. Failures always halt, even in autonomous mode — the one interruption.
//  5. After all bolts complete, stages 3.6 and 3.7 run once.
//
// Parallelism is dependency-driven, NOT mode-driven. Mode only controls whether
// the batch-level gate is presented (gated) or skipped (autonomous).
func (p *Pipeline) RunAllBolts(ctx context.Context, f *feature.Feature, onOutput OutputLineCallback) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	bolts, err := p.database.GetBolts(f.ID)
	if err != nil {
		return fmt.Errorf("getting bolts: %w", err)
	}
	if len(bolts) == 0 {
		return fmt.Errorf("no bolts prepared — call PrepareBolts first")
	}

	// 1. Walking skeleton — always sequential, always gated.
	ws := bolts[0]
	if ws.Status != "completed" {
		result, err := p.RunBolt(ctx, f, ws.BoltNumber, onOutput)
		if err != nil {
			return fmt.Errorf("walking skeleton: %w", err)
		}
		if result.Failed {
			log.Printf("RunAllBolts: walking skeleton failed — halting")
			return nil
		}
		if len(result.StageResults) > 0 {
			last := result.StageResults[len(result.StageResults)-1]
			if last.Gate != nil && last.Gate.IsOpen() {
				log.Printf("RunAllBolts: walking skeleton gate open — pausing for approval")
				return nil
			}
		}
	}

	// 2. Ladder prompt — block until the user (or human-proxy) sets the mode.
	if f.AutonomyMode == "" {
		log.Printf("RunAllBolts: waiting for ladder prompt resolution on feature %s", f.ID)
		if err := p.waitForLadderMode(ctx, f.ID); err != nil {
			return err
		}
		// Reload feature to pick up the autonomy mode set by LadderPrompt.
		latest, err := p.GetFeature(f.ID)
		if err != nil {
			return fmt.Errorf("reloading feature after ladder: %w", err)
		}
		f.AutonomyMode = latest.AutonomyMode
	}
	mode := f.AutonomyMode
	if mode == "" {
		// Spec: the ladder prompt fires exactly once after the walking-skeleton
		// gate and the user MUST pick gated or autonomous. If we get here with
		// no mode, the wait returned without a choice — that's a protocol
		// violation, not a safe-default situation. Surface it as an error so
		// the caller knows to re-trigger the ladder.
		return fmt.Errorf("ladder prompt not resolved for feature %s — user must choose gated or autonomous", f.ID)
	}

	// 3. Remaining bolts in dependency-ordered batches.
	remaining := bolts[1:]
	completed := make(map[int]bool, len(bolts))
	completed[ws.BoltNumber] = true

	for len(remaining) > 0 {
		// Find every bolt whose dependencies are all completed.
		var batch []db.BoltRow
		for _, b := range remaining {
			ready := true
			for _, dep := range b.DependsOn {
				if !completed[dep] {
					ready = false
					break
				}
			}
			if ready {
				batch = append(batch, b)
			}
		}
		if len(batch) == 0 {
			// No bolt is ready — either a cycle or all remaining bolts failed.
			log.Printf("RunAllBolts: no ready bolts (cycle or all failed) — halting")
			return nil
		}

		// Run the batch. Single bolt → sequential. Multiple → parallel.
		batchFailed := false
		if len(batch) == 1 {
			result, err := p.RunBolt(ctx, f, batch[0].BoltNumber, onOutput)
			if err != nil {
				return fmt.Errorf("bolt %d: %w", batch[0].BoltNumber, err)
			}
			if result.Failed {
				batchFailed = true
			}
			completed[batch[0].BoltNumber] = !result.Failed
		} else {
			var wg sync.WaitGroup
			var mu sync.Mutex
			failed := false
			for _, b := range batch {
				boltNum := b.BoltNumber
				wg.Add(1)
				go func() {
					defer wg.Done()
					result, err := p.RunBolt(ctx, f, boltNum, onOutput)
					if err != nil {
						mu.Lock()
						failed = true
						mu.Unlock()
						log.Printf("RunAllBolts: bolt %d errored: %v", boltNum, err)
						return
					}
					mu.Lock()
					if result.Failed {
						failed = true
					}
					completed[boltNum] = !result.Failed
					mu.Unlock()
				}()
			}
			wg.Wait()
			batchFailed = failed
		}

		// Spec: failures always halt, even in autonomous mode.
		if batchFailed {
			log.Printf("RunAllBolts: batch failed — halting (spec: failures always halt)")
			return nil
		}

		// Batch-level gate. Skipped in autonomous mode; presented in gated mode.
		if mode == AutonomyGated {
			if err := p.presentBatchGate(ctx, f, batch); err != nil {
				return err
			}
		}

		// Remove the batch from remaining.
		remaining = filterBolts(remaining, batch)
	}

	// 4. Once-at-end construction stages (3.6, 3.7).
	_, err = p.RunConstructionStages(ctx, f, onOutput)
	if err != nil {
		return fmt.Errorf("construction final stages: %w", err)
	}
	return nil
}

// waitForLadderMode blocks until the feature's AutonomyMode is set (via
// LadderPrompt, which the UI calls after the user picks gated/autonomous).
// Polls the DB at 2s intervals. Returns ctx.Err() on cancellation.
func (p *Pipeline) waitForLadderMode(ctx context.Context, featureID string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		f, err := p.GetFeature(featureID)
		if err != nil {
			return fmt.Errorf("waiting for ladder mode: %w", err)
		}
		if f.AutonomyMode != "" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// presentBatchGate waits for every bolt in the batch to have its 3.5 stage
// reach a terminal state (completed/failed/skipped). Each bolt's RunBolt
// already opened a per-bolt gate at 3.5; the user (or autonomous auto-approve)
// resolves each one. This method blocks until all are resolved.
// Autonomous mode never calls this — ProcessStageResult auto-approves there.
func (p *Pipeline) presentBatchGate(ctx context.Context, f *feature.Feature, batch []db.BoltRow) error {
	if len(batch) == 0 {
		return nil
	}
	stageID := "3.5"
	p.database.RecordAuditEvent(f.ID, db.AuditStageAwaitingApproval, stageID, stage.PhaseConstruction,
		fmt.Sprintf("batch gate covering bolts %v — awaiting per-bolt 3.5 approval", boltNumbers(batch)))
	p.broadcastSSE(f.ID, "batch_gate_awaiting",
		fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolts":%v}`, jsonString(f.ID), jsonString(stageID), boltNumbers(batch)))

	// Poll until every bolt in the batch has a terminal 3.5 status.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		allTerminal := true
		for _, b := range batch {
			fs, _ := p.database.GetFeatureStageForBolt(f.ID, stageID, b.BoltNumber)
			if fs == nil {
				// Row missing — bolt may not have reached 3.5 yet. Keep waiting.
				allTerminal = false
				break
			}
			switch fs.Status {
			case stage.StatusCompleted, stage.StatusSkipped, stage.StatusRevising:
				// terminal or being revised — count as resolved for batch flow
			default:
				allTerminal = false
			}
			if fs.Status == stage.StatusRevising {
				// A revision means the bolt is being re-run — not terminal.
				allTerminal = false
			}
		}
		if allTerminal {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// boltNumbers returns the bolt numbers from a slice of BoltRow, for logging.
func boltNumbers(bolts []db.BoltRow) []int {
	out := make([]int, len(bolts))
	for i, b := range bolts {
		out[i] = b.BoltNumber
	}
	return out
}

// filterBolts returns bolts from in that are not in remove.
func filterBolts(in, remove []db.BoltRow) []db.BoltRow {
	removed := make(map[int]bool, len(remove))
	for _, b := range remove {
		removed[b.BoltNumber] = true
	}
	out := in[:0]
	for _, b := range in {
		if !removed[b.BoltNumber] {
			out = append(out, b)
		}
	}
	return out
}
