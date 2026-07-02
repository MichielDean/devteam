package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/MichielDean/devteam/internal/db"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/gate"
	"github.com/MichielDean/devteam/internal/role"
	"github.com/MichielDean/devteam/internal/stage"
)

// sseBroadcaster is set by the API server to enable SSE broadcasts from the pipeline.
var sseBroadcaster SSEBroadcaster

// SetSSEBroadcaster sets the global SSE broadcaster for pipeline events.
// Called by the API server on initialization.
func SetSSEBroadcaster(b SSEBroadcaster) {
	sseBroadcaster = b
}

// broadcastSSE sends an event to the UI via SSE if a broadcaster is registered.
func (p *Pipeline) broadcastSSE(featureID string, eventType string, data string) {
	if sseBroadcaster != nil {
		sseBroadcaster.BroadcastSSE(featureID, eventType, data)
	}
}

// jsonString safely quotes a string for JSON embedding.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ReviewerMaxIterations is the maximum number of reviewer revision cycles.
const ReviewerMaxIterations = 2

// StageRunResult is the outcome of a single RunStage call.
type StageRunResult struct {
	StageID        string
	Phase          string
	StageName      string
	RoleResult     *role.DispatchResult
	SmokeFailures  []string
	Outcome        *db.OutcomeRow
	OutcomeSource  string // "agent_signal", "default_pass", "smoke_failed", "reviewer_rejected"
	Gate           *gate.Gate
	ReviewerResult *ReviewerResult
	Duration       time.Duration
}

// SSEBroadcaster is an interface for broadcasting SSE events to the UI.
// Implemented by the API server.
type SSEBroadcaster interface {
	BroadcastSSE(featureID string, eventType string, data string)
}

// ReviewerResult holds the outcome of a reviewer dispatch.
type ReviewerResult struct {
	Reviewer   string
	Verdict    string // "READY", "NOT-READY"
	Notes      string
	Iterations int
}

// RunStage dispatches the lead agent for one stage, waits for outcome,
// runs smoke check, dispatches reviewer if declared, and opens approval gate.
// Does NOT auto-advance — caller approves/rejects via gate API.
func (p *Pipeline) RunStage(ctx context.Context, f *feature.Feature, stageID string, onOutput OutputLineCallback) (*StageRunResult, error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("RunStage: PANIC recovered for feature %s stage %s: %v", f.ID, stageID, r)
		}
	}()

	if p.database == nil {
		return nil, fmt.Errorf("RunStage requires database")
	}

	stageDef, err := p.database.GetStageDefinition(stageID)
	if err != nil {
		return nil, fmt.Errorf("loading stage definition %s: %w", stageID, err)
	}

	log.Printf("RunStage: starting stage %s (%s) for feature %s", stageID, stageDef.Name, f.ID)

	now := time.Now()

	fs, err := p.database.GetFeatureStage(f.ID, stageID)
	if err != nil {
		return nil, fmt.Errorf("getting feature stage %s: %w", stageID, err)
	}
	if fs == nil {
		return nil, fmt.Errorf("feature stage %s not initialized for feature %s — call InitFeatureStages first", stageID, f.ID)
	}

	p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusInProgress, fs.RevisionCount, &now, nil)
	p.database.RecordAuditEvent(f.ID, db.AuditStageStart, stageID, stageDef.Phase, "")
	p.broadcastSSE(f.ID, "stage_started", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"phase":%s}`, jsonString(f.ID), jsonString(stageID), jsonString(stageDef.Phase)))

	// Update session state to running
	if p.sessionMgr != nil {
		boltNumber := 0
		if stageDef.Phase == stage.PhaseConstruction && f.CurrentBolt > 0 {
			boltNumber = f.CurrentBolt
		}
		p.sessionMgr.SetSessionRunning(f.ID, stageDef.Phase, boltNumber, stageID, stageDef.LeadAgent)
	}

	if err := p.EnsureSpecWorktree(f); err != nil {
		log.Printf("warning: could not create spec worktree: %v — using base dir", err)
	}

	contextStr, err := p.buildStageContext(ctx, f, stageDef)
	if err != nil {
		return nil, err
	}

	// Clean prior outcome for this stage
	p.database.DeleteOutcomesForPhase(f.ID, stageID)

	preDispatchCommit := p.recordGitCommit(f)

	roleDef, err := p.roleLoader.Load(stageDef.LeadAgent)
	if err != nil {
		return nil, fmt.Errorf("loading role %s: %w", stageDef.LeadAgent, err)
	}

	promptContext := roleDef.Instructions + "\n\n---\n\n" + contextStr
	promptContext += p.stageInstruction(stageDef, f)
	promptContext += stageOutcomeInstructions(stageDef)

	lineCh := make(chan role.OutputLine, 100)
	streamDone := make(chan struct{})
	if onOutput != nil {
		go func() {
			defer close(streamDone)
			for line := range lineCh {
				onOutput(line.Line, line.IsStderr)
			}
		}()
	} else {
		close(streamDone)
	}

	req := role.DispatchRequest{
		FeatureID:   f.ID,
		Phase:       stageDef.Phase,
		StageID:     stageID,
		Role:        stageDef.LeadAgent,
		Context:     promptContext,
		WorkingDir:  p.dispatchWorkingDirForStage(f, stageDef),
		SessionName: p.resolveSessionName(f, stageDef),
		ContextDir:  p.resolveContextDir(f, stageDef),
	}

	log.Printf("RunStage: dispatching agent %s for stage %s (session=%s)", stageDef.LeadAgent, stageID, req.SessionName)
	result, err := p.dispatcher.DispatchStreaming(ctx, req, lineCh)
	close(lineCh)
	<-streamDone
	if err != nil {
		p.database.UpdateFeatureStage(f.ID, stageID, "failed", fs.RevisionCount, &now, nil)
		p.broadcastSSE(f.ID, "error", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"message":%s}`, jsonString(f.ID), jsonString(stageID), jsonString(err.Error())))
		return nil, fmt.Errorf("dispatching agent %s for stage %s: %w", stageDef.LeadAgent, stageID, err)
	}

	outcomeSource := "default_pass"
	var outcome *db.OutcomeRow
	outcome, _ = p.database.GetLatestOutcome(f.ID, stageID)
	if outcome != nil {
		outcomeSource = "agent_signal"
		log.Printf("RunStage: agent outcome for %s: %s notes=%d chars", stageID, outcome.Outcome, len(outcome.Notes))
	} else {
		outcome = &db.OutcomeRow{FeatureID: f.ID, Phase: stageID, Outcome: "pass"}
		log.Printf("RunStage: no outcome signal — defaulting to pass for %s", stageID)
	}

	var smokeFailures []string
	if outcome.Outcome == "pass" {
		smokeFailures = p.stageSmokeCheck(f, stageDef, preDispatchCommit)
		if len(smokeFailures) > 0 {
			log.Printf("RunStage: smoke check failed for %s — %d failures", stageID, len(smokeFailures))
			outcomeSource = "smoke_failed"
		}
	}

	var reviewerResult *ReviewerResult
	if stageDef.Reviewer != "" && outcomeSource != "smoke_failed" && outcome.Outcome == "pass" {
		reviewerResult, err = p.dispatchReviewer(ctx, f, stageDef, onOutput)
		if err != nil {
			log.Printf("RunStage: reviewer dispatch failed for %s: %v", stageID, err)
		} else {
			p.recordReviewerAudit(f, stageDef, reviewerResult)
			p.broadcastSSE(f.ID, "gate_result", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"reviewer":%s,"verdict":%s,"notes":%s}`,
				jsonString(f.ID), jsonString(stageID), jsonString(reviewerResult.Reviewer), jsonString(reviewerResult.Verdict), jsonString(reviewerResult.Notes)))
			if reviewerResult.Verdict == "NOT-READY" {
				outcomeSource = "reviewer_rejected"
			}
		}
	}

	g := gate.New(f.ID, stageID)

	if outcomeSource == "smoke_failed" || outcomeSource == "reviewer_rejected" {
		p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusRevising, fs.RevisionCount, &now, nil)
		p.database.RecordAuditEvent(f.ID, db.AuditStageRevising, stageID, stageDef.Phase, strings.Join(smokeFailures, "; "))
		p.broadcastSSE(f.ID, "stage_revising", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"reason":%s}`, jsonString(f.ID), jsonString(stageID), jsonString(outcomeSource)))
		g.RevisionNotes = strings.Join(smokeFailures, "\n")
		if reviewerResult != nil && reviewerResult.Verdict == "NOT-READY" {
			g.RevisionNotes = reviewerResult.Notes
		}
		g.RevisionCount = fs.RevisionCount
	} else {
		p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusAwaitingApproval, fs.RevisionCount, &now, nil)
		p.database.RecordAuditEvent(f.ID, db.AuditStageAwaitingApproval, stageID, stageDef.Phase, "")
		p.broadcastSSE(f.ID, "stage_awaiting_approval", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"phase":%s}`, jsonString(f.ID), jsonString(stageID), jsonString(stageDef.Phase)))

		// Update session state to awaiting gate
		if p.sessionMgr != nil {
			boltNumber := 0
			if stageDef.Phase == stage.PhaseConstruction && f.CurrentBolt > 0 {
				boltNumber = f.CurrentBolt
			}
			p.sessionMgr.SetSessionAwaitingGate(f.ID, stageDef.Phase, boltNumber, stageID)
		}
	}

	if outcomeSource == "smoke_failed" || outcomeSource == "reviewer_rejected" {
		g.Reject(g.RevisionNotes)
	} else if outcome.Outcome == "pass" {
		// Gate open for user approval
	} else if outcome.Outcome == "failed" {
		p.database.UpdateFeatureStage(f.ID, stageID, "failed", fs.RevisionCount, &now, nil)
		p.broadcastSSE(f.ID, "error", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"message":"stage failed"}`, jsonString(f.ID), jsonString(stageID)))
	}

	p.database.AddNote(f.ID, stageID, stageDef.LeadAgent, "summary", outcome.Notes)

	result2 := &StageRunResult{
		StageID:        stageID,
		Phase:          stageDef.Phase,
		StageName:      stageDef.Name,
		RoleResult:     result,
		SmokeFailures:  smokeFailures,
		Outcome:        outcome,
		OutcomeSource:  outcomeSource,
		Gate:           g,
		ReviewerResult: reviewerResult,
		Duration:       time.Since(now),
	}

	log.Printf("RunStage: complete for %s stage %s (outcome=%s source=%s duration=%v)",
		f.ID, stageID, outcome.Outcome, outcomeSource, result2.Duration)

	return result2, nil
}

// dispatchReviewer fires a reviewer agent as an independent dispatch after the stage.
func (p *Pipeline) dispatchReviewer(ctx context.Context, f *feature.Feature, stageDef *db.StageDefinition, onOutput OutputLineCallback) (*ReviewerResult, error) {
	reviewerName := stageDef.Reviewer
	log.Printf("RunStage: dispatching reviewer %s for stage %s", reviewerName, stageDef.ID)

	roleDef, err := p.roleLoader.Load(reviewerName)
	if err != nil {
		return nil, fmt.Errorf("loading reviewer role %s: %w", reviewerName, err)
	}

	reviewContext := roleDef.Instructions + "\n\n---\n\n"
	reviewContext += fmt.Sprintf("# Review Request\n\nFeature: %s\nStage: %s (%s)\nPhase: %s\n\n", f.ID, stageDef.ID, stageDef.Name, stageDef.Phase)
	reviewContext += "Review the artifacts produced by this stage. Produce a verdict: READY or NOT-READY.\n"
	reviewContext += "If NOT-READY, list specific findings with what's wrong and what good looks like.\n\n"
	reviewContext += "Signal your verdict:\n"
	reviewContext += fmt.Sprintf("  devteam signal %s pass --notes \"READY: brief reason\" — if the stage output is sound\n", f.ID)
	reviewContext += fmt.Sprintf("  devteam signal %s recirculate:%s --notes \"NOT-READY: specific findings\" — if the stage output needs fixes\n", f.ID, stageDef.ID)

	lineCh := make(chan role.OutputLine, 100)
	streamDone := make(chan struct{})
	if onOutput != nil {
		go func() {
			defer close(streamDone)
			for line := range lineCh {
				onOutput(line.Line, line.IsStderr)
			}
		}()
	} else {
		close(streamDone)
	}

	req := role.DispatchRequest{
		FeatureID:  f.ID,
		Phase:      stageDef.Phase,
		StageID:    stageDef.ID + "-review",
		Role:       reviewerName,
		Context:    reviewContext,
		WorkingDir: p.WorktreeDir(f),
	}

	reviewResult, err := p.dispatcher.DispatchStreaming(ctx, req, lineCh)
	close(lineCh)
	<-streamDone
	if err != nil {
		return nil, fmt.Errorf("dispatching reviewer %s: %w", reviewerName, err)
	}
	_ = reviewResult

	reviewerOutcome, _ := p.database.GetLatestOutcome(f.ID, stageDef.ID+"-review")
	verdict := "READY"
	notes := ""
	if reviewerOutcome != nil {
		notes = reviewerOutcome.Notes
		if reviewerOutcome.Outcome == "recirculate" {
			verdict = "NOT-READY"
		}
	} else {
		notes = "Reviewer did not signal — defaulting to READY"
	}

	return &ReviewerResult{
		Reviewer:   reviewerName,
		Verdict:    verdict,
		Notes:      notes,
		Iterations: 1, // single pass; revision cycles tracked via feature_stages.revision_count
	}, nil
}

// reviewerIterationsExceeded checks if the reviewer has exceeded max iterations
// for this stage (based on revision_count in feature_stages).
func (p *Pipeline) reviewerIterationsExceeded(f *feature.Feature, stageID string) bool {
	if p.database == nil {
		return false
	}
	fs, err := p.database.GetFeatureStage(f.ID, stageID)
	if err != nil || fs == nil {
		return false
	}
	return fs.RevisionCount >= ReviewerMaxIterations
}

// recordReviewerAudit records the reviewer dispatch result.
func (p *Pipeline) recordReviewerAudit(f *feature.Feature, stageDef *db.StageDefinition, result *ReviewerResult) {
	if p.database == nil || result == nil {
		return
	}
	p.database.RecordAuditEvent(f.ID, db.AuditSubagentCompleted, stageDef.ID, stageDef.Phase,
		fmt.Sprintf("reviewer=%s verdict=%s", result.Reviewer, result.Verdict))
}

// ApproveStage approves the gate for a stage and advances to the next stage.
func (p *Pipeline) ApproveStage(f *feature.Feature, stageID string) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	fs, err := p.database.GetFeatureStage(f.ID, stageID)
	if err != nil || fs == nil {
		return fmt.Errorf("feature stage %s not found", stageID)
	}

	// Only allow approving stages that are awaiting approval
	if fs.Status != stage.StatusAwaitingApproval {
		return fmt.Errorf("stage %s is in %s state — can only approve stages that are awaiting_approval", stageID, fs.Status)
	}

	now := time.Now().UTC()
	p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
	p.database.RecordAuditEvent(f.ID, db.AuditGateApproved, stageID, "", "")
	p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
	p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))
	p.broadcastSSE(f.ID, "gate_approved", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))

	return p.AdvanceStage(f, stageID)
}

// RejectStage rejects the gate, saves rejection notes as a rule (learning loop),
// and sets the stage to revising state.
func (p *Pipeline) RejectStage(f *feature.Feature, stageID, rejectionNotes string) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	fs, err := p.database.GetFeatureStage(f.ID, stageID)
	if err != nil || fs == nil {
		return fmt.Errorf("feature stage %s not found", stageID)
	}

	// Only allow rejecting stages that are awaiting approval or already revising
	if fs.Status != stage.StatusAwaitingApproval && fs.Status != stage.StatusRevising {
		return fmt.Errorf("stage %s is in %s state — can only reject stages that are awaiting_approval or revising", stageID, fs.Status)
	}

	stageDef, err := p.database.GetStageDefinition(stageID)
	if err != nil {
		return fmt.Errorf("loading stage definition: %w", err)
	}

	newRevisionCount := fs.RevisionCount + 1
	p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusRevising, newRevisionCount, fs.StartedAt, nil)

	if p.database != nil {
		p.database.AddNote(f.ID, stageID, stageDef.LeadAgent, "revision", rejectionNotes)
		p.database.RecordAuditEvent(f.ID, db.AuditGateRejected, stageID, "", rejectionNotes)
		p.database.RecordAuditEvent(f.ID, db.AuditRuleLearned, stageID, "", rejectionNotes)
		p.broadcastSSE(f.ID, "gate_rejected", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s,"notes":%s}`, jsonString(f.ID), jsonString(stageID), jsonString(rejectionNotes)))

		// Learning loop: save rejection as a rule for this agent
		ruleText := fmt.Sprintf("Stage %s rejection: %s", stageID, rejectionNotes)
		p.database.SaveRule(f.ID, stageDef.LeadAgent, stageID, ruleText, rejectionNotes)
	}

	return nil
}

// AcceptStageAsIs uses the 3-strike escape hatch to accept a stage despite issues.
func (p *Pipeline) AcceptStageAsIs(f *feature.Feature, stageID string) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	fs, err := p.database.GetFeatureStage(f.ID, stageID)
	if err != nil || fs == nil {
		return fmt.Errorf("feature stage %s not found", stageID)
	}

	if fs.RevisionCount < gate.MaxRevisions {
		return fmt.Errorf("accept-as-is requires %d revisions (current: %d)", gate.MaxRevisions, fs.RevisionCount)
	}

	now := time.Now().UTC()
	p.database.UpdateFeatureStage(f.ID, stageID, stage.StatusCompleted, fs.RevisionCount, fs.StartedAt, &now)
	p.database.RecordAuditEvent(f.ID, db.AuditGateAcceptAsIs, stageID, "", "")
	p.database.RecordAuditEvent(f.ID, db.AuditStageCompleted, stageID, "", "")
	p.broadcastSSE(f.ID, "stage_completed", fmt.Sprintf(`{"feature_id":%s,"stage_id":%s}`, jsonString(f.ID), jsonString(stageID)))

	return p.AdvanceStage(f, stageID)
}

// AdvanceStage moves to the next stage in the feature's scope, respecting conditions.
// If the current stage is the last, marks the feature as done.
func (p *Pipeline) AdvanceStage(f *feature.Feature, currentStageID string) error {
	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}

	stages, err := p.database.GetStagesForScope(scope)
	if err != nil {
		return fmt.Errorf("getting stages for scope %s: %w", scope, err)
	}

	currentIdx := -1
	for i, s := range stages {
		if s.ID == currentStageID {
			currentIdx = i
			break
		}
	}
	if currentIdx < 0 {
		return fmt.Errorf("stage %s not found in scope %s", currentStageID, scope)
	}
	if currentIdx >= len(stages)-1 {
		// Last stage — mark feature done
		log.Printf("AdvanceStage: stage %s is the last stage for feature %s", currentStageID, f.ID)
		if p.database != nil {
			p.database.RecordAuditEvent(f.ID, db.AuditWorkflowComplete, "", "", "")
		}
		return nil
	}

	nextStage := stages[currentIdx+1]

	// Check condition — skip stages that don't apply
	for p.shouldSkipStage(f, nextStage) {
		p.database.UpdateFeatureStage(f.ID, nextStage.ID, stage.StatusSkipped, 0, nil, nil)
		p.database.RecordAuditEvent(f.ID, db.AuditStageSkipped, nextStage.ID, nextStage.Phase, nextStage.Condition)
		currentIdx++
		if currentIdx >= len(stages)-1 {
			log.Printf("AdvanceStage: reached end after skipping stages for feature %s", f.ID)
			if p.database != nil {
				p.database.RecordAuditEvent(f.ID, db.AuditWorkflowComplete, "", "", "")
			}
			return nil
		}
		nextStage = stages[currentIdx+1]
	}

	// Update feature's current stage pointer
	if p.database != nil {
		p.database.UpdateFeatureStage(f.ID, nextStage.ID, stage.StatusNotStarted, 0, nil, nil)
		p.database.RecordAuditEvent(f.ID, db.AuditStageAdvanced, nextStage.ID, nextStage.Phase, "")
	}

	log.Printf("AdvanceStage: advanced from %s to %s for feature %s", currentStageID, nextStage.ID, f.ID)
	return nil
}

// shouldSkipStage evaluates whether a CONDITIONAL stage should be skipped.
func (p *Pipeline) shouldSkipStage(f *feature.Feature, s db.StageDefinition) bool {
	switch s.Condition {
	case stage.CondAlways:
		return false
	case stage.CondConditional, stage.CondUserFacing, stage.CondUIProject, stage.CondBrownfield, stage.CondPerBolt, stage.CondOnceAtEnd:
		// Already filtered by scope — if it's in the scope's stage set, it runs.
		// Brownfield/user-facing conditions are handled at scope detection time.
		return false
	}
	return false
}

// JumpToStage jumps to a specific stage, marking intervening stages as skipped.
func (p *Pipeline) JumpToStage(f *feature.Feature, targetStageID string) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}

	stages, err := p.database.GetStagesForScope(scope)
	if err != nil {
		return fmt.Errorf("getting stages for scope: %w", err)
	}

	targetIdx := -1
	for i, s := range stages {
		if s.ID == targetStageID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return fmt.Errorf("stage %s not found in scope %s", targetStageID, scope)
	}

	// Find current stage
	currentIdx := -1
	for i, s := range stages {
		fs, _ := p.database.GetFeatureStage(f.ID, s.ID)
		if fs != nil && (fs.Status == stage.StatusInProgress || fs.Status == stage.StatusAwaitingApproval) {
			currentIdx = i
			break
		}
	}

	// Mark intervening stages as skipped
	startIdx := 0
	if currentIdx >= 0 {
		startIdx = currentIdx + 1
	}
	for i := startIdx; i < targetIdx; i++ {
		p.database.UpdateFeatureStage(f.ID, stages[i].ID, stage.StatusSkipped, 0, nil, nil)
	}

	p.database.UpdateFeatureStage(f.ID, targetStageID, stage.StatusNotStarted, 0, nil, nil)
	if p.database != nil {
		p.database.RecordAuditEvent(f.ID, db.AuditJumpToStage, targetStageID, "", fmt.Sprintf("skipped %d stages", targetIdx-startIdx))
	}

	log.Printf("JumpToStage: jumped to %s for feature %s (skipped %d stages)", targetStageID, f.ID, targetIdx-startIdx)
	return nil
}

// JumpToPhase jumps to the first stage of a phase.
func (p *Pipeline) JumpToPhase(f *feature.Feature, targetPhase string) error {
	if p.database == nil {
		return fmt.Errorf("database required")
	}

	scope := f.Scope
	if scope == "" {
		scope = stage.ScopeFeature
	}

	stages, err := p.database.GetStagesForScope(scope)
	if err != nil {
		return fmt.Errorf("getting stages: %w", err)
	}

	for _, s := range stages {
		if s.Phase == targetPhase {
			return p.JumpToStage(f, s.ID)
		}
	}
	return fmt.Errorf("phase %s not found in scope %s", targetPhase, scope)
}

// buildStageContext assembles context for a stage dispatch, including team knowledge and rules.
func (p *Pipeline) buildStageContext(ctx context.Context, f *feature.Feature, stageDef *db.StageDefinition) (string, error) {
	contextStr := fmt.Sprintf("# Stage: %s (%s)\n\nPhase: %s\nScope: %s\nDepth: %s\n\n",
		stageDef.Name, stageDef.ID, stageDef.Phase, f.Scope, f.Depth)

	// Key artifacts this stage should produce
	if len(stageDef.KeyArtifacts) > 0 {
		contextStr += "## Key Artifacts to Produce\n\n"
		for _, a := range stageDef.KeyArtifacts {
			contextStr += fmt.Sprintf("- %s\n", a)
		}
		contextStr += "\n"
	}

	// Supporting agents info
	if len(stageDef.SupportingAgents) > 0 {
		contextStr += "## Supporting Agents\n\n"
		contextStr += "These agents may provide input. The orchestrator handles coordination — you do not invoke them directly.\n\n"
		for _, a := range stageDef.SupportingAgents {
			contextStr += fmt.Sprintf("- %s\n", a)
		}
		contextStr += "\n"
	}

	// Reviewer info
	if stageDef.Reviewer != "" {
		contextStr += fmt.Sprintf("## Reviewer\n\nThis stage will be reviewed by **%s** after you complete. Ensure your output meets review criteria.\n\n", stageDef.Reviewer)
	}

	// Feature input
	if stageDef.Phase == stage.PhaseIdeation || stageDef.Phase == stage.PhaseInception {
		if inputContent, err := p.specProvider.ReadArtifact(f.ID, feature.ArtifactInputMD); err == nil && inputContent != "" {
			contextStr += "\n\n---\n\n=== Feature Input ===\n" + inputContent
		}
	}

	// Questions + human responses
	if p.questionStore != nil {
		questions, qErr := p.questionStore.ListQuestions(ctx, f.ID)
		if qErr == nil && len(questions) > 0 {
			timeoutMinutes := p.config.Pipeline.GetHumanInteractionTimeoutMinutes()
			humanResponses := feature.BuildHumanResponsesContext(questions, timeoutMinutes)
			if humanResponses != "" {
				contextStr += humanResponses
			}
		}
	}

	// Spec cross-repo context
	specContext, err := p.specProvider.BuildCrossRepoContext(f.ID, nil)
	if err == nil && specContext != "" {
		contextStr += "\n\n---\n\n" + specContext
	}

	// Revision notes from prior rejections
	if p.database != nil {
		revisionNotes := p.buildStageRevisionNotes(f, stageDef.ID)
		if revisionNotes != "" {
			contextStr += revisionNotes
		}

		notesContext := p.database.BuildNotesContext(f.ID, stageDef.ID)
		if notesContext != "" {
			contextStr += notesContext
		}

		// Team knowledge injection
		knowledge, _ := p.database.GetTeamKnowledge(stageDef.LeadAgent)
		knowledgeEntries := make([]role.TeamKnowledgeEntry, len(knowledge))
		for i, k := range knowledge {
			knowledgeEntries[i] = role.TeamKnowledgeEntry{Topic: k.Topic, Content: k.Content}
		}

		// Learned rules injection
		rules, _ := p.database.GetRulesForAgent(stageDef.LeadAgent, f.ID)
		ruleEntries := make([]role.RuleEntry, len(rules))
		for i, r := range rules {
			ruleEntries[i] = role.RuleEntry{StageID: r.StageID, RuleText: r.RuleText}
		}

		contextStr += role.FormatKnowledgeAndRules(knowledgeEntries, ruleEntries)
	}

	return contextStr, nil
}

func (p *Pipeline) buildStageRevisionNotes(f *feature.Feature, stageID string) string {
	if p.database == nil {
		return ""
	}
	notes, err := p.database.GetNotesForPhase(f.ID, stageID)
	if err != nil || len(notes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n\n# ⚠️ REVISION REQUIRED\n\n")
	b.WriteString("A previous run of this stage was sent back. Address these issues:\n\n")
	for _, n := range notes {
		if n.NoteType == "revision" {
			b.WriteString(fmt.Sprintf("## From %s\n\n%s\n\n", n.Role, n.Content))
		}
	}
	return b.String()
}

// stageInstruction returns stage-specific instructions for the agent.
func (p *Pipeline) stageInstruction(stageDef *db.StageDefinition, f *feature.Feature) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n\n---\n\n## Stage Instructions\n\n"))
	b.WriteString(fmt.Sprintf("You are executing stage **%s (%s)** for feature **%s**.\n\n", stageDef.Name, stageDef.ID, f.ID))
	b.WriteString(fmt.Sprintf("Phase: %s\n", stageDef.Phase))
	b.WriteString(fmt.Sprintf("Depth: %s — ", f.Depth))
	switch f.Depth {
	case stage.DepthMinimal:
		b.WriteString("Core essentials, 1-2 page artifacts, key decisions only\n\n")
	case stage.DepthStandard:
		b.WriteString("Complete artifacts, all required sections, concise rationale\n\n")
	case stage.DepthComprehensive:
		b.WriteString("Full enterprise detail, compliance matrices, exhaustive NFRs\n\n")
	}

	b.WriteString("## Submitting Artifacts\n\n")
	b.WriteString(fmt.Sprintf("Spec artifacts are stored in the database, NOT on disk. Submit via CLI:\n"))
	b.WriteString(fmt.Sprintf("  devteam artifact submit %s <type> --file <filename>\n", f.ID))
	b.WriteString(fmt.Sprintf("  devteam artifact submit %s <type> --content \"inline content\"\n\n", f.ID))

	if len(stageDef.KeyArtifacts) > 0 {
		b.WriteString("Key artifacts for this stage:\n")
		for _, a := range stageDef.KeyArtifacts {
			b.WriteString(fmt.Sprintf("- %s\n", a))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Asking Questions\n\n")
	b.WriteString(fmt.Sprintf("If you need clarification, ask via CLI:\n  devteam questions ask %s --file questions.json\n", f.ID))
	b.WriteString(fmt.Sprintf("Then signal: devteam signal %s needs_feedback\n\n", f.ID))

	b.WriteString("## Implementation Repositories\n\n")
	b.WriteString("If this stage involves code changes, the implementation repo worktrees are listed in CONTEXT.md. Commit in those worktrees, not the spec repo.\n")

	return b.String()
}

// stageOutcomeInstructions tells the agent how to signal completion.
func stageOutcomeInstructions(stageDef *db.StageDefinition) string {
	var b strings.Builder
	b.WriteString("\n\n---\n\n## Outcome Signal (MANDATORY)\n\n")
	b.WriteString("After completing your work, signal your outcome:\n\n")
	b.WriteString(fmt.Sprintf("- `devteam signal <feature-id> pass` — your work is complete and verified\n"))
	b.WriteString(fmt.Sprintf("- `devteam signal <feature-id> recirculate:%s --notes \"what needs fixing\"` — send back for revision\n", stageDef.ID))
	b.WriteString(fmt.Sprintf("- `devteam signal <feature-id> needs_feedback` — you submitted questions\n"))
	b.WriteString(fmt.Sprintf("- `devteam signal <feature-id> failed --notes \"why\"` — you are blocked\n\n"))
	b.WriteString("If you don't signal, the pipeline assumes `pass`.\n")
	return b.String()
}

// stageSmokeCheck runs phase-appropriate smoke checks for a stage.
func (p *Pipeline) stageSmokeCheck(f *feature.Feature, stageDef *db.StageDefinition, preDispatchCommit string) []string {
	switch stageDef.Phase {
	case stage.PhaseConstruction:
		if stageDef.ID == "3.5" || stageDef.ID == "3.6" {
			return p.smokeImplFilesChanged(f, preDispatchCommit)
		}
	}
	// For non-construction stages, no smoke check — the reviewer handles quality
	return nil
}

func (p *Pipeline) dispatchWorkingDirForStage(f *feature.Feature, stageDef *db.StageDefinition) string {
	if stageDef.Phase == stage.PhaseConstruction || stageDef.Phase == stage.PhaseOperation {
		if dirs := p.implRepoDirs(f); len(dirs) > 0 {
			return dirs[0]
		}
	}
	return p.WorktreeDir(f)
}

// resolveSessionName returns the tmux session name for a stage dispatch.
// For construction stages, uses per-Bolt session if BoltNumber is set on the feature.
// For other phases, uses per-phase session.
func (p *Pipeline) resolveSessionName(f *feature.Feature, stageDef *db.StageDefinition) string {
	if p.sessionMgr == nil {
		// Fallback: derive directly from tmux manager
		tmuxMgr := p.dispatcher.TmuxManager()
		return tmuxMgr.SessionNameForPhase(f.ID, stageDef.Phase)
	}

	boltNumber := 0
	if stageDef.Phase == stage.PhaseConstruction && f.CurrentBolt > 0 {
		boltNumber = f.CurrentBolt
	}

	sessionName, _, err := p.sessionMgr.ResolveOrCreateSession(f.ID, stageDef.Phase, boltNumber)
	if err != nil {
		log.Printf("resolveSessionName: failed to resolve session: %v — deriving directly", err)
		tmuxMgr := p.dispatcher.TmuxManager()
		if boltNumber > 0 {
			return tmuxMgr.SessionNameForBolt(f.ID, boltNumber)
		}
		return tmuxMgr.SessionNameForPhase(f.ID, stageDef.Phase)
	}
	return sessionName
}

// resolveContextDir returns the persistent context directory for a stage dispatch.
func (p *Pipeline) resolveContextDir(f *feature.Feature, stageDef *db.StageDefinition) string {
	tmuxMgr := p.dispatcher.TmuxManager()
	boltNumber := 0
	if stageDef.Phase == stage.PhaseConstruction && f.CurrentBolt > 0 {
		boltNumber = f.CurrentBolt
	}
	if boltNumber > 0 {
		return tmuxMgr.ContextDirForBolt(f.ID, boltNumber)
	}
	return tmuxMgr.ContextDirForPhase(f.ID, stageDef.Phase)
}