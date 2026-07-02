package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
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

// PrepareBolts reads units-of-work from inception output (stage 2.7) and creates
// Bolt records in the bolts table. Bolt 1 is the walking skeleton.
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

	// Bolt 1 = walking skeleton (first unit or smallest unit)
	boltUnits := p.groupUnitsIntoBolts(units)
	for i, bu := range boltUnits {
		isWalkingSkeleton := i == 0
		if err := p.database.CreateBolt(f.ID, i+1, bu, isWalkingSkeleton); err != nil {
			return fmt.Errorf("creating bolt %d: %w", i+1, err)
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
// ponytail: one unit per Bolt for now. Parallel batching is future debt.
func (p *Pipeline) groupUnitsIntoBolts(units []string) [][]string {
	bolts := make([][]string, len(units))
	for i, u := range units {
		bolts[i] = []string{u}
	}
	return bolts
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
		// Check if this stage applies to the feature's scope
		stageDef, err := p.database.GetStageDefinition(stageID)
		if err != nil {
			log.Printf("RunBolt: skipping stage %s — definition not found: %v", stageID, err)
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

		// If gate is open (awaiting approval), stop and return
		if stageResult.Gate != nil && stageResult.Gate.IsOpen() {
			log.Printf("RunBolt: stage %s opened gate — pausing bolt %d", stageID, boltNumber)
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
	p.database.UpdateBoltStatus(f.ID, boltNumber, "completed")
	p.database.RecordAuditEvent(f.ID, db.AuditBoltCompleted, "", stage.PhaseConstruction,
		fmt.Sprintf("bolt %d completed", boltNumber))

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

// RunAllBolts runs all Bolts in sequence, then the once-at-end construction stages.
// This is the full construction phase entry point after PrepareBolts.
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

	for _, bolt := range bolts {
		result, err := p.RunBolt(ctx, f, bolt.BoltNumber, onOutput)
		if err != nil {
			return fmt.Errorf("bolt %d: %w", bolt.BoltNumber, err)
		}

		if result.Failed {
			// Halt-and-ask: return to user for retry/skip/abort decision
			log.Printf("RunAllBolts: bolt %d failed at stage %s — halting for user decision", bolt.BoltNumber, result.FailureStage)
			return nil
		}

		// If gate opened, pause for approval
		if len(result.StageResults) > 0 {
			last := result.StageResults[len(result.StageResults)-1]
			if last.Gate != nil && last.Gate.IsOpen() {
				log.Printf("RunAllBolts: bolt %d gate open — pausing for approval", bolt.BoltNumber)
				return nil
			}
		}

		// After walking skeleton (bolt 1), fire ladder prompt if not yet answered
		if bolt.IsWalkingSkeleton && f.AutonomyMode == "" {
			log.Printf("RunAllBolts: walking skeleton complete — ladder prompt needed for feature %s", f.ID)
			// In gated mode (default), continue with gates. User can change via API.
			f.AutonomyMode = AutonomyGated
			p.saveFeatureState(f)
			p.database.RecordAuditEvent(f.ID, db.AuditLadderPrompt, "", stage.PhaseConstruction,
				"defaulting to gated autonomy (user can change via API)")
		}

		// In autonomous mode, skip per-Bolt gate (already handled above — no gate opened)
	}

	// Run once-at-end construction stages (3.6, 3.7)
	_, err = p.RunConstructionStages(ctx, f, onOutput)
	if err != nil {
		return fmt.Errorf("construction final stages: %w", err)
	}

	return nil
}