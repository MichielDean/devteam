package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

// ProcessEvent represents an event emitted during autonomous processing.
// Defined in the pipeline package to avoid circular imports with the api package.
type ProcessEvent struct {
	Type       string          `json:"type"` // "phase_change", "gate_result", "agent_dispatch", "agent_complete", "processing_complete", "error"
	FeatureID  string          `json:"feature_id"`
	Phase      feature.Phase   `json:"phase"`
	Status     string          `json:"status,omitempty"`
	Passed     bool            `json:"passed,omitempty"`
	Checks     []CheckResult   `json:"checks,omitempty"`
	Role       string          `json:"role,omitempty"`
	DurationMs int64           `json:"duration_ms,omitempty"`
	Message    string          `json:"message,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Timestamp  time.Time       `json:"timestamp"`
}

// CheckResult is the pipeline-level representation of a gate check result for ProcessEvent
type CheckResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// ProcessAsync runs the autonomous processing loop, emitting events to the provided channel.
// It loops through phases until delivery or max recirculations, emitting events at each step.
func (p *Pipeline) ProcessAsync(ctx context.Context, f *feature.Feature, eventCh chan<- ProcessEvent) error {
	maxRecirculations := 3
	recirculationCount := 0

	// Ensure feature is in progress
	if f.Status == feature.StatusDraft {
		f.Start()
		if err := p.specProvider.SaveFeatureState(f); err != nil {
			return fmt.Errorf("setting feature to in_progress: %w", err)
		}
	}

	// Create feature branch and draft PR at pipeline start
	branchCreated := false
	if _, err := p.gitClient.CurrentBranch(); err == nil {
		// Already on a branch — check if we need to create a feature branch
		branchName := "feat/" + f.ID
		if !p.gitClient.HasRemoteBranch(branchName) {
			if _, err := p.CreateFeatureBranch(f); err != nil {
				log.Printf("warning: could not create feature branch: %v", err)
			} else {
				branchCreated = true
			}
		} else {
			branchCreated = true
		}
	}

	now := time.Now()

	// Emit initial phase_change event
	eventCh <- ProcessEvent{
		Type:      "phase_change",
		FeatureID: f.ID,
		Phase:     f.Current,
		Status:    string(f.Status),
		Timestamp: now,
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// If feature is waiting for human input, check if all questions are answered
		if f.Status == feature.StatusWaitingHuman {
			pendingCount, err := p.questionStore.PendingCount(ctx, f.ID)
			if err != nil {
				log.Printf("error checking pending questions for feature %s: %v", f.ID, err)
			}
			if pendingCount == 0 {
				// All questions answered, resume pipeline
				if err := f.ResumeFromWaitingHuman(); err != nil {
					log.Printf("error resuming feature %s from waiting_for_human: %v", f.ID, err)
				} else {
					if err := p.specProvider.SaveFeatureState(f); err != nil {
						log.Printf("error saving feature state for %s: %v", f.ID, err)
					}
					eventCh <- ProcessEvent{
						Type:      "questions_answered",
						FeatureID: f.ID,
						Phase:     f.Current,
						Status:    string(f.Status),
						Message:   "All questions answered, resuming pipeline",
						Timestamp: time.Now(),
					}
				}
				// Reload feature state
				f, err = p.GetFeature(f.ID)
				if err != nil {
					return fmt.Errorf("reloading feature state: %w", err)
				}
			} else {
				// Still waiting for human input, emit event and wait
				// In a real implementation, this would pause and wait for SSE events
				// For now, we check on the next loop iteration
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
					// Check again
					f, err := p.GetFeature(f.ID)
					if err != nil {
						return fmt.Errorf("reloading feature state: %w", err)
					}
					_ = f
					continue
				}
			}
		}

		currentPhase := f.Current

		// Emit agent_dispatch event
		eventCh <- ProcessEvent{
			Type:      "agent_dispatch",
			FeatureID: f.ID,
			Phase:     currentPhase,
			Role:      p.primaryRole(currentPhase),
			Status:    "dispatched",
			Timestamp: time.Now(),
		}

		// Run the phase
		_, err := p.RunPhaseWithAgent(ctx, f)
		if err != nil {
			eventCh <- ProcessEvent{
				Type:      "error",
				FeatureID: f.ID,
				Phase:     currentPhase,
				Message:   fmt.Sprintf("Phase %s execution failed: %v", currentPhase, err),
				Timestamp: time.Now(),
			}
			return fmt.Errorf("phase %s execution failed: %w", currentPhase, err)
		}

		// Emit agent_complete event
		eventCh <- ProcessEvent{
			Type:       "agent_complete",
			FeatureID:  f.ID,
			Phase:      currentPhase,
			Role:       p.primaryRole(currentPhase),
			Status:     "success",
			DurationMs: int64(time.Since(now).Milliseconds()),
			Timestamp:  time.Now(),
		}

		// Check for questions after inception/planning phases
		if currentPhase == feature.PhaseInception || currentPhase == feature.PhasePlanning {
			timeoutMinutes := p.config.Pipeline.GetHumanInteractionTimeoutMinutes()
			detectedQuestions := feature.DetectQuestions(f.ID, p.specProvider.FeatureDir(f.ID))

			if len(detectedQuestions) > 0 {
				// Store detected questions
				for i := range detectedQuestions {
					detectedQuestions[i].FeatureID = f.ID
					if _, err := p.questionStore.CreateQuestion(ctx, f.ID, detectedQuestions[i]); err != nil {
						log.Printf("warning: failed to create question for feature %s: %v", f.ID, err)
						continue
					}
				}

				// Check if we should pause for human input
				// Reload feature to get latest state after agent dispatch
				f, err = p.GetFeature(f.ID)
				if err != nil {
					return fmt.Errorf("reloading feature state after question detection: %w", err)
				}

				if feature.ShouldPauseForHuman(f, timeoutMinutes) {
					// Pause for human input
					if err := f.WaitForHuman(); err != nil {
						log.Printf("warning: cannot transition feature %s to waiting_for_human: %v", f.ID, err)
					} else {
						if err := p.specProvider.SaveFeatureState(f); err != nil {
							log.Printf("warning: failed to save feature state for %s: %v", f.ID, err)
						}
						eventCh <- ProcessEvent{
							Type:      "waiting_for_human",
							FeatureID: f.ID,
							Phase:     f.Current,
							Status:    string(f.Status),
							Message:   "Pipeline paused for human input",
							Timestamp: time.Now(),
						}

						// Start timeout goroutine for auto-assume
						if timeoutMinutes > 0 {
							go p.startTimeoutGoroutine(ctx, f.ID, timeoutMinutes)
						}

						// Don't proceed to gate evaluation - wait for human input
						// Reload and continue the loop, which will check waiting_for_human status
						f, err = p.GetFeature(f.ID)
						if err != nil {
							return fmt.Errorf("reloading feature state: %w", err)
						}
						now = time.Now()
						continue
					}
				} else if timeoutMinutes == 0 {
					// Fully autonomous mode - immediately assume all questions
					_, err := feature.AssumeAllPendingQuestions(p.questionStore, f.ID, timeoutMinutes)
					if err != nil {
						log.Printf("warning: failed to auto-assume questions for feature %s: %v", f.ID, err)
					}
					// Inject human responses into context on next dispatch
				}
			}
		}

		// Evaluate gate
		gr, err := p.EvaluateGate(f)
		if err != nil {
			eventCh <- ProcessEvent{
				Type:      "error",
				FeatureID: f.ID,
				Phase:     currentPhase,
				Message:   fmt.Sprintf("Gate evaluation failed: %v", err),
				Timestamp: time.Now(),
			}
			return fmt.Errorf("gate evaluation for phase %s: %w", currentPhase, err)
		}

		// Emit gate_result event
		checks := make([]CheckResult, 0, len(gr.Checks))
		for _, c := range gr.Checks {
			checks = append(checks, CheckResult{
				Name:    c.Name,
				Passed:  c.Passed,
				Message: c.Message,
			})
		}
		eventCh <- ProcessEvent{
			Type:      "gate_result",
			FeatureID: f.ID,
			Phase:     currentPhase,
			Passed:    gr.Passed,
			Checks:    checks,
			Timestamp: time.Now(),
		}

		if gr.Passed {
			// Push phase changes after gate passes
			if branchCreated {
				if err := p.PushPhaseChanges(f, currentPhase); err != nil {
					log.Printf("warning: could not push phase changes: %v", err)
				}
			}

			// Check if we've reached delivery
			if currentPhase == feature.PhaseDelivery {
				f.MarkDone()
				if err := p.specProvider.SaveFeatureState(f); err != nil {
					return fmt.Errorf("marking feature done: %w", err)
				}

				// Mark draft PR as ready for review
				if branchCreated {
					if err := p.MarkPRReady(f); err != nil {
						log.Printf("warning: could not mark PR ready: %v", err)
					}
				}

				eventCh <- ProcessEvent{
					Type:      "processing_complete",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Status:    string(f.Status),
					Timestamp: time.Now(),
				}
				return nil
			}

			// Advance to next phase
			f, err = p.AdvanceFeature(f)
			if err != nil {
				eventCh <- ProcessEvent{
					Type:      "error",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Message:   fmt.Sprintf("Failed to advance: %v", err),
					Timestamp: time.Now(),
				}
				return fmt.Errorf("advancing from %s: %w", currentPhase, err)
			}

			eventCh <- ProcessEvent{
				Type:      "phase_change",
				FeatureID: f.ID,
				Phase:     f.Current,
				Status:    string(f.Status),
				Timestamp: time.Now(),
			}
		} else {
			// Gate failed — recirculate
			recirculationCount++
			if recirculationCount > maxRecirculations {
				eventCh <- ProcessEvent{
					Type:      "error",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Message:   fmt.Sprintf("Maximum recirculations (%d) reached for phase %s", maxRecirculations, currentPhase),
					Timestamp: time.Now(),
				}
				return fmt.Errorf("maximum recirculations reached for feature %s", f.ID)
			}

			// Determine recirculation target
			targetPhase := feature.RecirculationTarget(currentPhase, "gate_failed")

			// Clear questions on recirculation
			if p.questionStore != nil {
				if err := p.questionStore.DeleteQuestionsForFeature(ctx, f.ID); err != nil {
					log.Printf("warning: failed to delete questions for feature %s on recirculate: %v", f.ID, err)
				}
			}

			f, err = p.RecirculateFeature(f, targetPhase, "gate failed during autonomous processing")
			if err != nil {
				eventCh <- ProcessEvent{
					Type:      "error",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Message:   fmt.Sprintf("Failed to recirculate: %v", err),
					Timestamp: time.Now(),
				}
				return fmt.Errorf("recirculating from %s to %s: %w", currentPhase, targetPhase, err)
			}

			eventCh <- ProcessEvent{
				Type:      "phase_change",
				FeatureID: f.ID,
				Phase:     f.Current,
				Status:    string(f.Status),
				Timestamp: time.Now(),
			}
		}

		// Reload feature state
		f, err = p.GetFeature(f.ID)
		if err != nil {
			return fmt.Errorf("reloading feature state: %w", err)
		}

		now = time.Now()
	}
}

// startTimeoutGoroutine starts a goroutine that auto-assumes pending questions after a timeout.
func (p *Pipeline) startTimeoutGoroutine(ctx context.Context, featureID string, timeoutMinutes int) {
	timer := time.NewTimer(time.Duration(timeoutMinutes) * time.Minute)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		// Timeout expired — auto-assume all pending questions
		assumed, err := feature.AssumeAllPendingQuestions(p.questionStore, featureID, timeoutMinutes)
		if err != nil {
			log.Printf("error auto-assuming questions for feature %s: %v", featureID, err)
			return
		}

		if len(assumed) > 0 {
			// Transition feature back to in_progress
			f, err := p.GetFeature(featureID)
			if err != nil {
				log.Printf("error reloading feature %s after timeout: %v", featureID, err)
				return
			}

			if f.Status == feature.StatusWaitingHuman {
				if err := f.ResumeFromWaitingHuman(); err != nil {
					log.Printf("error resuming feature %s from waiting_for_human: %v", featureID, err)
					return
				}
				if err := p.specProvider.SaveFeatureState(f); err != nil {
					log.Printf("error saving feature %s state after timeout: %v", featureID, err)
					return
				}

				p.broadcastSSE(featureID, "questions_assumed", fmt.Sprintf(`{"feature_id":"%s","assumed_count":%d}`, featureID, len(assumed)))
			}
		}
	}
}

// broadcastSSE sends an SSE event for a feature. This is a helper that
// uses the Pipeline's Server reference if available, or is a no-op otherwise.
// The actual SSE broadcasting is done through the Server's broadcastSSE method,
// so we need a way to connect the Pipeline to the Server.
// For now, we log the event. The Server's processFeature handler already broadcasts events.
func (p *Pipeline) broadcastSSE(featureID string, eventType string, data string) {
	// SSE broadcasting is handled by the API Server's processFeature handler
	// through the eventCh channel. This method is a placeholder for direct
	// pipeline-initiated events.
	log.Printf("SSE event: type=%s feature=%s data=%s", eventType, featureID, data)
}

// primaryRole returns the first role configured for the given phase
func (p *Pipeline) primaryRole(phase feature.Phase) string {
	phaseConfig, err := p.getPhaseConfig(phase)
	if err != nil || len(phaseConfig.Roles) == 0 {
		return string(phase)
	}
	return phaseConfig.Roles[0]
}
