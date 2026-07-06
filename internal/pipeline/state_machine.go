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
	OutcomeNeedsReview    StageOutcome = iota // gate open, human must approve
	OutcomeAutoApproved                       // auto-approved, ready to advance
	OutcomeFailed                             // stage failed, human intervention needed
	OutcomeComplete                           // no more stages, feature is done
)

// ProcessStageResult is the SINGLE function that decides what happens after a stage runs.
// It handles gate decisions, auto-approval, SSE broadcasts, and advancement.
// All callers (runStageAsync, recoverStage, bolts) must use this — no duplicate logic.
//
// Returns the outcome and the next stage ID to run (if auto-advancing).
// The caller is responsible for actually running the next stage (calling RunStage).
func (p *Pipeline) ProcessStageResult(f *feature.Feature, stageID string, result *StageRunResult) StageOutcome {
	if result == nil {
		return OutcomeFailed
	}

	now := time.Now()
	fs, _ := p.database.GetFeatureStage(f.ID, stageID)
	if fs == nil {
		return OutcomeFailed
	}

	// Save agent output to DB
	if result.RoleResult != nil && result.RoleResult.Output != "" {
		p.database.SaveStageLog(f.ID, stageID, result.StageName, result.RoleResult.Output)
	}

	// Check for failures first
	if result.OutcomeSource == "smoke_failed" || result.OutcomeSource == "reviewer_rejected" || result.OutcomeSource == "agent_failed" {
		p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusRevising, fs.RevisionCount, &now, nil)
		p.database.RecordAuditEvent(f.ID, "STAGE_FAILED", stageID, "", result.RoleResult.Error)
		p.broadcastSSE(f.ID, "stage_revising", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"reason":%s}`, jsonString(f.ID), jsonString(stageID), jsonString(result.OutcomeSource)))
		return OutcomeFailed
	}

	// Check if gate is open (needs approval)
	if result.Gate == nil || !result.Gate.IsOpen() {
		// Gate was auto-approved inside RunStage (autonomous/guided/init)
		// or stage completed without a gate
		p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
		p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
		p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))
		return OutcomeAutoApproved
	}

	// Gate is open — check if we should auto-approve based on execution mode
	stageDef, _ := p.database.GetStageDefinition(stageID)
	isInitStage := stageDef != nil && stageDef.Phase == stage.PhaseInitialization
	isPhaseEndGate := stageDef != nil && (stageID == "1.7" || stageID == "2.8" || stageID == "3.7" || stageID == "4.7" || (stageDef.Phase == stage.PhaseConstruction && stageID == "3.5"))

	mode := f.ExecutionMode
	if mode == "" {
		mode = ExecutionHuman
	}

	shouldAutoApprove := isInitStage || mode == ExecutionAutonomous || (mode == ExecutionGuided && !isPhaseEndGate)

	if shouldAutoApprove {
		// Auto-approve
		p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
		p.database.RecordAuditEvent(f.ID, db.AuditGateApproved, stageID, "", fmt.Sprintf("auto-approved (%s mode)", mode))
		p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
		p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))
		p.broadcastSSE(f.ID, "gate_approved", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"auto":true}`, jsonString(f.ID), jsonString(stageID)))
		return OutcomeAutoApproved
	}

	// Gate open, needs human review
	p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusAwaitingApproval, fs.RevisionCount, fs.StartedAt, &now)
	p.database.RecordAuditEvent(f.ID, db.AuditStageAwaitingApproval, stageID, "", "")
	p.broadcastSSE(f.ID, "stage_awaiting_approval", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))
	return OutcomeNeedsReview
}

// NextStageToRun finds the next not_started stage for a feature.
// Returns empty string if no more stages exist.
func (p *Pipeline) NextStageToRun(featureID string) string {
	stages, err := p.database.GetFeatureStages(featureID)
	if err != nil {
		return ""
	}
	for _, s := range stages {
		if s.Status == stage.StatusNotStarted {
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
func (p *Pipeline) ApproveAndAdvance(f *feature.Feature, stageID string) error {
	fs, err := p.database.GetFeatureStage(f.ID, stageID)
	if err != nil || fs == nil {
		return fmt.Errorf("feature stage %s not found", stageID)
	}

	// Only allow approving from awaiting_approval or revising
	if fs.Status != stage.StatusAwaitingApproval && fs.Status != stage.StatusRevising {
		return fmt.Errorf("stage %s is in %s state — can only approve awaiting_approval or revising", stageID, fs.Status)
	}

	// Check reviewer rejection
	outcome, _ := p.database.GetLatestOutcome(f.ID, stageID)
	if outcome != nil && outcome.Outcome == "recirculate" {
		return fmt.Errorf("stage %s was rejected by reviewer — re-run before approving", stageID)
	}

	now := time.Now().UTC()
	p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
	p.database.RecordAuditEvent(f.ID, db.AuditGateApproved, stageID, "", "")
	p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
	p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))
	p.broadcastSSE(f.ID, "gate_approved", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))

	return p.AdvanceStage(f, stageID)
}