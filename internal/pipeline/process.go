package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/MichielDean/devteam/internal/feature"
)

// ProcessEvent represents an event emitted during autonomous processing.
// Defined in the pipeline package to avoid circular imports with the api package.
type ProcessEvent struct {
	Type       string          `json:"type"`        // "phase_change", "gate_result", "agent_dispatch", "agent_complete", "processing_complete", "error"
	FeatureID string          `json:"feature_id"`
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

	now := time.Now()

	// Emit initial phase_change event
	eventCh <- ProcessEvent{
		Type:       "phase_change",
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

		currentPhase := f.Current

		// Emit agent_dispatch event
		eventCh <- ProcessEvent{
			Type:       "agent_dispatch",
			FeatureID: f.ID,
			Phase:     currentPhase,
			Role:       p.primaryRole(currentPhase),
			Status:    "dispatched",
			Timestamp: time.Now(),
		}

		// Run the phase
		_, err := p.RunPhaseWithAgent(ctx, f)
		if err != nil {
			eventCh <- ProcessEvent{
				Type:       "error",
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
			FeatureID: f.ID,
			Phase:     currentPhase,
			Role:       p.primaryRole(currentPhase),
			Status:    "success",
			DurationMs: int64(time.Since(now).Milliseconds()),
			Timestamp: time.Now(),
		}

		// Evaluate gate
		gr, err := p.EvaluateGate(f)
		if err != nil {
			eventCh <- ProcessEvent{
				Type:       "error",
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
			Type:       "gate_result",
			FeatureID: f.ID,
			Phase:     currentPhase,
			Passed:    gr.Passed,
			Checks:     checks,
			Timestamp: time.Now(),
		}

		if gr.Passed {
			// Check if we've reached delivery
			if currentPhase == feature.PhaseDelivery {
				f.MarkDone()
				if err := p.specProvider.SaveFeatureState(f); err != nil {
					return fmt.Errorf("marking feature done: %w", err)
				}

				eventCh <- ProcessEvent{
					Type:       "processing_complete",
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
					Type:       "error",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Message:   fmt.Sprintf("Failed to advance: %v", err),
					Timestamp: time.Now(),
				}
				return fmt.Errorf("advancing from %s: %w", currentPhase, err)
			}

			eventCh <- ProcessEvent{
				Type:       "phase_change",
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
					Type:       "error",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Message:   fmt.Sprintf("Maximum recirculations (%d) reached for phase %s", maxRecirculations, currentPhase),
					Timestamp: time.Now(),
				}
				return fmt.Errorf("maximum recirculations reached for feature %s", f.ID)
			}

			// Determine recirculation target
			targetPhase := feature.RecirculationTarget(currentPhase, "gate_failed")
			f, err = p.RecirculateFeature(f, targetPhase, "gate failed during autonomous processing")
			if err != nil {
				eventCh <- ProcessEvent{
					Type:       "error",
					FeatureID: f.ID,
					Phase:     currentPhase,
					Message:   fmt.Sprintf("Failed to recirculate: %v", err),
					Timestamp: time.Now(),
				}
				return fmt.Errorf("recirculating from %s to %s: %w", currentPhase, targetPhase, err)
			}

			eventCh <- ProcessEvent{
				Type:       "phase_change",
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

// primaryRole returns the first role configured for the given phase
func (p *Pipeline) primaryRole(phase feature.Phase) string {
	phaseConfig, err := p.getPhaseConfig(phase)
	if err != nil || len(phaseConfig.Roles) == 0 {
		return string(phase)
	}
	return phaseConfig.Roles[0]
}