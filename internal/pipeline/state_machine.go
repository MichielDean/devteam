package pipeline

import (
	"fmt"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/stage"
)

// StageOutcome is the result of processing a completed stage.
// It tells the caller what happened and what to do next.
type StageOutcome int

const (
	OutcomeNeedsReview  StageOutcome = iota // gate open, human must approve
	OutcomeAutoApproved                     // auto-approved, ready to advance
	OutcomeFailed                           // stage failed, human intervention needed
	OutcomeComplete                         // no more stages, feature is done
)

// ProcessStageResult is the SINGLE function that decides what happens after a stage runs.
// It handles gate decisions, auto-approval, SSE broadcasts, and advancement.
// All callers (runStageAsync, recoverStage, bolts) must use this — no duplicate logic.
//
// The bolt number is read from result.BoltNumber (0 for non-construction stages).
// Returns the outcome. The caller is responsible for running the next stage.
func (p *Pipeline) ProcessStageResult(f *feature.Feature, stageID string, result *StageRunResult) StageOutcome {
	if result == nil {
		return OutcomeFailed
	}

	boltNumber := result.BoltNumber
	now := time.Now()
	fs, _ := p.getFeatureStageRow(f.ID, stageID, boltNumber)
	if fs == nil {
		return OutcomeFailed
	}

	// Save agent output to DB
	if result.RoleResult != nil && result.RoleResult.Output != "" {
		p.database.SaveStageLog(f.ID, stageID, result.StageName, result.RoleResult.Output)
	}

	// Check for failures first
	if result.OutcomeSource == "smoke_failed" || result.OutcomeSource == "reviewer_rejected" || result.OutcomeSource == "agent_failed" {
		p.updateFeatureStageRow(f.ID, stageID, boltNumber, stage.StatusRevising, fs.RevisionCount, &now, nil)
		p.database.RecordAuditEvent(f.ID, "STAGE_FAILED", stageID, "", result.RoleResult.Error)
		p.broadcastSSE(f.ID, "stage_revising", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d,"reason":%s}`, jsonString(f.ID), jsonString(stageID), boltNumber, jsonString(result.OutcomeSource)))
		return OutcomeFailed
	}

	// Check if gate is open (needs approval)
	if result.Gate == nil || !result.Gate.IsOpen() {
		// Gate was auto-approved inside RunStage (autonomous/guided/init)
		// or stage completed without a gate
		p.updateFeatureStageRow(f.ID, stageID, boltNumber, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
		p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
		p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d}`, jsonString(f.ID), jsonString(stageID), boltNumber))
		return OutcomeAutoApproved
	}

	// Gate is open — check if we should auto-approve based on execution mode
	stageDef, _ := p.database.GetStageDefinition(stageID)
	isInitStage := stageDef != nil && stageDef.Phase == stage.PhaseInitialization
	// Per-Bolt stages (3.1-3.5) are gated at the Bolt/batch level by RunAllBolts,
	// not at the individual stage level. So 3.5 is NOT a phase-end gate here;
	// the Bolt-level gate replaces it (AIDLC v2 spec).
	isPhaseEndGate := stageDef != nil && (stageID == "1.7" || stageID == "2.8" || stageID == "3.7" || stageID == "4.7")

	mode := f.ExecutionMode
	if mode == "" {
		mode = ExecutionHuman
	}

	shouldAutoApprove := isInitStage || mode == ExecutionAutonomous || (mode == ExecutionGuided && !isPhaseEndGate)

	if shouldAutoApprove {
		// Auto-approve
		p.updateFeatureStageRow(f.ID, stageID, boltNumber, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
		p.database.RecordAuditEvent(f.ID, db.AuditGateApproved, stageID, "", fmt.Sprintf("auto-approved (%s mode)", mode))
		p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
		p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d}`, jsonString(f.ID), jsonString(stageID), boltNumber))
		p.broadcastSSE(f.ID, "gate_approved", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d,"auto":true}`, jsonString(f.ID), jsonString(stageID), boltNumber))
		return OutcomeAutoApproved
	}

	// Gate open, needs human review
	p.updateFeatureStageRow(f.ID, stageID, boltNumber, stage.StatusAwaitingApproval, fs.RevisionCount, fs.StartedAt, &now)
	p.database.RecordAuditEvent(f.ID, db.AuditStageAwaitingApproval, stageID, "", "")
	p.broadcastSSE(f.ID, "stage_awaiting_approval", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d}`, jsonString(f.ID), jsonString(stageID), boltNumber))
	return OutcomeNeedsReview
}

// NextStageToRun finds the next not_started NON-per-Bolt stage for a feature,
// skipping any stages that are not in the feature's scope.
// Per-Bolt construction stages (3.1-3.5) are driven by RunAllBolts/RunBolt,
// NOT by this function — they are skipped here so the auto-advance loop in
// runStageAsync doesn't try to run them out of bolt order.
// Marks skipped stages as skipped in the DB and records audit events.
// Returns empty string if no more stages exist.
func (p *Pipeline) NextStageToRun(featureID string) string {
	stages, err := p.database.GetFeatureStages(featureID)
	if err != nil {
		return ""
	}
	scope := ""
	if f, err := p.GetFeature(featureID); err == nil {
		scope = f.Scope
	}
	for _, s := range stages {
		// Skip per-Bolt stages — they're driven by RunAllBolts/RunBolt.
		if isPerBoltStageID(s.StageID) {
			continue
		}
		if s.Status == stage.StatusNotStarted {
			// Check if this stage should be skipped based on scope
			stageDef, _ := p.database.GetStageDefinition(s.StageID)
			if stageDef != nil && p.ShouldSkipStage(&feature.Feature{Scope: scope}, *stageDef) {
				now := time.Now()
				p.database.UpdateFeatureStage(featureID, s.StageID, stage.StatusSkipped, 0, &now, nil)
				p.database.RecordAuditEvent(featureID, db.AuditStageSkipped, s.StageID, stageDef.Phase, fmt.Sprintf("not in scope %q", scope))
				p.broadcastSSE(featureID, "stage_skipped", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(featureID), jsonString(s.StageID)))
				continue // skip this stage, check the next one
			}
			return s.StageID
		}
	}
	return ""
}

// ShouldAutoAdvance checks if the pipeline should automatically advance
// to the next stage after a stage was auto-approved.
func (p *Pipeline) ShouldAutoAdvance(f *feature.Feature, stageID string) bool {
	stageDef, _ := p.database.GetStageDefinition(stageID)
	isInitStage := stageDef != nil && stageDef.Phase == stage.PhaseInitialization
	mode := f.ExecutionMode
	if mode == "" {
		mode = ExecutionHuman
	}
	return isInitStage || mode == ExecutionGuided || mode == ExecutionAutonomous
}

// ApproveAndAdvance approves a stage gate and advances to the next stage.
// Used by the approve API handler and by auto-approval flows.
// boltNumber is 0 for non-construction stages; 1+ for per-Bolt stages.
// Returns the next NON-per-Bolt stage ID to run (empty if no more stages).
func (p *Pipeline) ApproveAndAdvance(f *feature.Feature, stageID string, boltNumber int) (string, error) {
	fs, err := p.getFeatureStageRow(f.ID, stageID, boltNumber)
	if err != nil || fs == nil {
		return "", fmt.Errorf("feature stage %s (bolt %d) not found", stageID, boltNumber)
	}

	// Only allow approving from awaiting_approval or revising
	if fs.Status != stage.StatusAwaitingApproval && fs.Status != stage.StatusRevising {
		return "", fmt.Errorf("stage %s (bolt %d) is in %s state — can only approve awaiting_approval or revising", stageID, boltNumber, fs.Status)
	}

	// Check reviewer rejection
	outcome, _ := p.database.GetLatestOutcome(f.ID, stageID)
	if outcome != nil && outcome.Outcome == "recirculate" {
		return "", fmt.Errorf("stage %s was rejected by reviewer — re-run before approving", stageID)
	}

	now := time.Now().UTC()
	p.updateFeatureStageRow(f.ID, stageID, boltNumber, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
	p.database.RecordAuditEvent(f.ID, db.AuditGateApproved, stageID, "", "")
	p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
	p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d}`, jsonString(f.ID), jsonString(stageID), boltNumber))
	p.broadcastSSE(f.ID, "gate_approved", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"bolt":%d}`, jsonString(f.ID), jsonString(stageID), boltNumber))

	// AdvanceStage only applies to non-per-Bolt stages. Per-Bolt stages
	// are sequenced by RunAllBolts, not by the linear AdvanceStage logic.
	if !isPerBoltStageID(stageID) {
		if err := p.AdvanceStage(f, stageID); err != nil {
			return "", err
		}
	}

	// Return the next NON-per-Bolt stage to run
	return p.NextStageToRun(f.ID), nil
}
