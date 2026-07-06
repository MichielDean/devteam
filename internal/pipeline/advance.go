package pipeline

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/stage"
)

// AdvanceFeature is the SINGLE control flow for advancing a feature through
// the AIDLC pipeline. One function, one loop, one state machine. All other
// entry points (runStageAsync, runBolt, approveStage, recovery) delegate here.
//
// The loop:
//  1. Load feature state + all stage rows
//  2. Decide what to do next based on state
//  3. Do it (run stage, auto-approve, prepare bolts, merge to main)
//  4. Repeat until: feature done, needs human review, or error
//
// Idempotent: safe to call repeatedly. If the feature is already being
// advanced by another goroutine, returns immediately (caller checks
// isFeatureActive before calling). If the feature is in a state that
// needs no action (all stages done or waiting for human), returns.
//
// Context cancellation stops the loop gracefully.
func (p *Pipeline) AdvanceFeature(ctx context.Context, featureID string, onOutput OutputLineCallback) error {
	f, err := p.GetFeature(featureID)
	if err != nil {
		return fmt.Errorf("loading feature: %w", err)
	}

	mode := f.ExecutionMode
	if mode == "" {
		mode = ExecutionHuman
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Reload feature state each iteration — it may have changed.
		f, err = p.GetFeature(featureID)
		if err != nil {
			return fmt.Errorf("reloading feature: %w", err)
		}

		stages, err := p.database.GetFeatureStages(featureID)
		if err != nil {
			return fmt.Errorf("loading stages: %w", err)
		}

		action := p.decideNextAction(f, stages, mode)

		switch action.kind {
		case actionDone:
			log.Printf("AdvanceFeature: %s complete — no more actions", featureID)
			p.markFeatureDone(featureID)
			return nil

		case actionWait:
			// Stage is in_progress (being run by a tmux session) or
			// revising (needs human or re-run). Nothing to do.
			log.Printf("AdvanceFeature: %s waiting — %s", featureID, action.reason)
			return nil

		case actionNeedsHuman:
			// Gate open in human mode — stop and let the user approve.
			log.Printf("AdvanceFeature: %s needs human review at stage %s", featureID, action.stageID)
			return nil

		case actionAutoApprove:
			// Stage is awaiting_approval and mode is autonomous/guided — approve it.
			log.Printf("AdvanceFeature: %s auto-approving stage %s bolt %d", featureID, action.stageID, action.boltNumber)
			if _, err := p.ApproveStage(f, action.stageID, action.boltNumber); err != nil {
				log.Printf("AdvanceFeature: auto-approve failed for %s stage %s: %v", featureID, action.stageID, err)
				return err
			}
			continue // loop to find next action

		case actionPrepareBolts:
			// 2.8 just completed — prepare bolts from units-of-work.
			log.Printf("AdvanceFeature: %s preparing bolts after 2.8", featureID)
			if err := p.PrepareBolts(f); err != nil {
				log.Printf("AdvanceFeature: PrepareBolts failed for %s: %v", featureID, err)
				return err
			}
			continue // loop to find next action

		case actionRunBolt:
			// Run the next incomplete bolt (3.1-3.5 per-bolt stages).
			log.Printf("AdvanceFeature: %s running bolt %d", featureID, action.boltNumber)
			result, err := p.RunBolt(ctx, f, action.boltNumber, onOutput)
			if err != nil {
				return fmt.Errorf("bolt %d: %w", action.boltNumber, err)
			}
			if result.Failed {
				log.Printf("AdvanceFeature: bolt %d failed at %s — halting", action.boltNumber, result.FailureStage)
				return nil
			}
			// RunBolt handles auto-approve internally; if it paused at a gate
			// (human mode), stop.
			if result.PausedAtGate() {
				log.Printf("AdvanceFeature: bolt %d paused at gate — waiting for human", action.boltNumber)
				return nil
			}
			continue // loop to find next action (next bolt or 3.6)

		case actionMergeToMain:
			// All construction complete (3.1-3.7) — merge to main.
			// Record MERGE_ATTEMPTED immediately so we don't loop on failure.
			log.Printf("AdvanceFeature: %s merging to main", featureID)
			p.broadcastSSE(featureID, "merging_to_main", fmt.Sprintf(`{"feature_id":%s}`, jsonString(featureID)))
			p.database.RecordAuditEvent(featureID, "MERGE_ATTEMPTED", "", "", "construction complete")
			if err := p.MergeFeatureToMain(f); err != nil {
				log.Printf("AdvanceFeature: merge to main failed for %s: %v — continuing to operation", featureID, err)
			} else {
				p.database.RecordAuditEvent(featureID, "MERGED_TO_MAIN", "", "", "merge succeeded")
			}
			continue // loop to find next action (operation phase)

		case actionRunStage:
			// Run a linear (non-per-bolt) stage.
			log.Printf("AdvanceFeature: %s running stage %s", featureID, action.stageID)
			result, err := p.RunStage(ctx, f, action.stageID, onOutput)
			if err != nil {
				return fmt.Errorf("stage %s: %w", action.stageID, err)
			}
			outcome := p.ProcessStageResult(f, action.stageID, result)
			switch outcome {
			case OutcomeNeedsReview:
				return nil // human must approve
			case OutcomeFailed:
				return nil // stage failed, halt
			case OutcomeComplete:
				p.markFeatureDone(featureID)
				return nil
			}
			// OutcomeAutoApproved — check if we should auto-advance
			if !p.ShouldAutoAdvance(f, action.stageID) {
				return nil // human mode — stop after each stage
			}
			time.Sleep(2 * time.Second)
			continue // loop to find next action
		}
	}
}

// nextAction is what AdvanceFeature decides to do next.
type nextAction struct {
	kind       actionKind
	stageID    string
	boltNumber int
	reason     string
}

type actionKind int

const (
	actionDone         actionKind = iota // feature is complete
	actionWait                           // stage in progress or revising
	actionNeedsHuman                     // gate open, human mode
	actionAutoApprove                    // awaiting_approval, auto-approve
	actionPrepareBolts                   // 2.8 done, prepare bolts
	actionRunBolt                        // run next incomplete bolt
	actionMergeToMain                    // 3.7 done, merge to main
	actionRunStage                       // run a linear stage
)

// decideNextAction examines the feature's stage state and decides what to do.
// This is the brain of the pipeline — one place that understands the full flow.
//
// Priority order:
//  1. In-progress stages → wait
//  2. Awaiting-approval stages → auto-approve (autonomous/guided) or needs-human
//  3. Revising stages → wait (needs re-run or human intervention)
//  4. 2.8 completed + no bolts → prepare bolts
//  5. Bolts with incomplete per-bolt stages → run bolt
//  6. 3.7 completed + not merged → merge to main
//  7. Next not-started linear stage → run it
//  8. No stages left → done
func (p *Pipeline) decideNextAction(f *feature.Feature, stages []db.FeatureStage, mode string) nextAction {
	// 1. Check for in-progress stages (someone is running).
	for _, s := range stages {
		if s.Status == stage.StatusInProgress {
			return nextAction{kind: actionWait, reason: fmt.Sprintf("stage %s bolt %d in progress", s.StageID, s.BoltNumber)}
		}
	}

	// 2. Check for awaiting-approval stages.
	for _, s := range stages {
		if s.Status == stage.StatusAwaitingApproval {
			if mode == ExecutionAutonomous || mode == ExecutionGuided {
				return nextAction{kind: actionAutoApprove, stageID: s.StageID, boltNumber: s.BoltNumber}
			}
			return nextAction{kind: actionNeedsHuman, stageID: s.StageID, reason: "awaiting approval in human mode"}
		}
	}

	// 3. Check for revising stages (failed, needs re-run or human).
	for _, s := range stages {
		if s.Status == stage.StatusRevising {
			// In autonomous/guided mode, re-run the stage automatically.
			if mode == ExecutionAutonomous || mode == ExecutionGuided {
				if isPerBoltStageID(s.StageID) && s.BoltNumber > 0 {
					return nextAction{kind: actionRunBolt, boltNumber: s.BoltNumber, reason: fmt.Sprintf("bolt %d stage %s revising — re-run", s.BoltNumber, s.StageID)}
				}
				return nextAction{kind: actionRunStage, stageID: s.StageID, reason: "stage revising — re-run"}
			}
			return nextAction{kind: actionWait, reason: fmt.Sprintf("stage %s revising — needs human", s.StageID)}
		}
	}

	// 4. 2.8 completed + no bolts → prepare bolts.
	stage28Done := false
	hasBoltsPrepared := false
	for _, s := range stages {
		if s.StageID == "2.8" && s.Status == stage.StatusCompleted {
			stage28Done = true
		}
	}
	if stage28Done {
		bolts, _ := p.database.GetBolts(f.ID)
		if len(bolts) == 0 {
			return nextAction{kind: actionPrepareBolts, reason: "2.8 done, no bolts"}
		}
		hasBoltsPrepared = true
	}

	// 5. Bolts with incomplete per-bolt stages → run next incomplete bolt.
	if hasBoltsPrepared {
		bolts, _ := p.database.GetBolts(f.ID)
		for _, b := range bolts {
			if b.Status == "completed" {
				continue
			}
			// Check if this bolt has any not-started per-bolt stages.
			hasNotStarted := false
			for _, s := range stages {
				if s.BoltNumber == b.BoltNumber && s.Status == stage.StatusNotStarted && isPerBoltStageID(s.StageID) {
					hasNotStarted = true
					break
				}
			}
			if hasNotStarted {
				return nextAction{kind: actionRunBolt, boltNumber: b.BoltNumber, reason: fmt.Sprintf("bolt %d has incomplete stages", b.BoltNumber)}
			}
		}
	}

	// 6. 3.7 completed → merge to main (check audit log to avoid re-merging).
	stage37Done := false
	for _, s := range stages {
		if s.StageID == "3.7" && s.Status == stage.StatusCompleted {
			stage37Done = true
			break
		}
	}
	if stage37Done {
		// Check if we already attempted merge (success or failure).
		var mergeCount int
		err := p.database.QueryRow(
			`SELECT COUNT(*) FROM audit_events WHERE feature_id = ? AND event_type IN ('MERGED_TO_MAIN', 'MERGE_ATTEMPTED')`, f.ID).Scan(&mergeCount)
		if err == nil && mergeCount == 0 {
			return nextAction{kind: actionMergeToMain, reason: "3.7 done, not yet merged"}
		}
	}

	// 7. Next not-started linear stage → run it.
	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}
	for _, s := range stages {
		if s.Status != stage.StatusNotStarted {
			continue
		}
		// Skip per-bolt stages — handled by bolt loop above.
		if isPerBoltStageID(s.StageID) {
			continue
		}
		// Check scope.
		stageDef, _ := p.database.GetStageDefinition(s.StageID)
		if stageDef != nil && p.ShouldSkipStage(&feature.Feature{Scope: scope}, *stageDef) {
			now := time.Now()
			p.database.UpdateFeatureStage(f.ID, s.StageID, stage.StatusSkipped, 0, &now, nil)
			p.database.RecordAuditEvent(f.ID, db.AuditStageSkipped, s.StageID, stageDef.Phase, fmt.Sprintf("not in scope %q", scope))
			p.broadcastSSE(f.ID, "stage_skipped", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(s.StageID)))
			continue
		}
		return nextAction{kind: actionRunStage, stageID: s.StageID, reason: "next linear stage"}
	}

	// 8. No stages left → done.
	return nextAction{kind: actionDone, reason: "all stages complete or skipped"}
}

// markFeatureDone sets the feature status to done and broadcasts SSE.
func (p *Pipeline) markFeatureDone(featureID string) {
	f, err := p.GetFeature(featureID)
	if err != nil {
		return
	}
	f.Status = feature.StatusDone
	p.SaveFeature(f)
	p.database.RecordAuditEvent(featureID, "FEATURE_DONE", "", "", "all stages complete")
	p.broadcastSSE(featureID, "feature_done", fmt.Sprintf(`{"feature_id":%s}`, jsonString(featureID)))
}
