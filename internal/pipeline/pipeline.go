package pipeline

import (
	"fmt"
	"time"

	"github.com/MichielDean/devteam/internal/config"
	"github.com/MichielDean/devteam/internal/feature"
	"github.com/MichielDean/devteam/internal/spec"
)

type Pipeline struct {
	config       *config.Config
	specProvider *spec.SpecProvider
}

func NewPipeline(cfg *config.Config, specProvider *spec.SpecProvider) *Pipeline {
	return &Pipeline{
		config:       cfg,
		specProvider: specProvider,
	}
}

func (p *Pipeline) RunPhase(f *feature.Feature) (*feature.PhaseState, error) {
	currentPhase := f.CurrentPhase()
	phaseConfig, err := p.getPhaseConfig(currentPhase)
	if err != nil {
		return nil, err
	}

	roles := phaseConfig.Roles
	if len(roles) == 0 {
		return nil, fmt.Errorf("no roles configured for phase %s", currentPhase)
	}

	now := time.Now()
	ps := f.PhaseStates[currentPhase]
	if ps == nil {
		ps = &feature.PhaseState{
			Phase: currentPhase,
		}
		f.PhaseStates[currentPhase] = ps
	}
	ps.Status = feature.StatusInProgress
	ps.StartedAt = &now

	return ps, nil
}

func (p *Pipeline) EvaluateGate(f *feature.Feature) (*feature.GateResult, error) {
	currentPhase := f.CurrentPhase()
	requiredArts := feature.RequiredArtifactsForPhase(currentPhase)
	result := p.specProvider.ValidateArtifacts(f.ID, requiredArts)
	result.Phase = currentPhase

	gateChecks := feature.GetGateDefinition(currentPhase)
	if gateChecks != nil {
		for _, desc := range gateChecks.ValidationDescs {
			result.Checks = append(result.Checks, feature.CheckResult{
				Name:    desc,
				Passed:  result.Passed,
				Message: fmt.Sprintf("gate check: %s", desc),
			})
		}
	}

	return &result, nil
}

func (p *Pipeline) getPhaseConfig(phase feature.Phase) (*config.PhaseConfig, error) {
	for i := range p.config.Pipeline.Phases {
		if p.config.Pipeline.Phases[i].Name == string(phase) {
			return &p.config.Pipeline.Phases[i], nil
		}
	}
	return nil, fmt.Errorf("phase %s not found in config", phase)
}

func (p *Pipeline) ListFeatures() ([]*feature.Feature, error) {
	return p.specProvider.ListFeatures()
}

func (p *Pipeline) GetFeature(featureID string) (*feature.Feature, error) {
	return p.specProvider.LoadFeatureState(featureID)
}

func (p *Pipeline) AdvanceFeature(f *feature.Feature, targetPhase feature.Phase) error {
	return f.AdvanceTo(targetPhase)
}

func (p *Pipeline) RecirculateFeature(f *feature.Feature, targetPhase feature.Phase, reason string) error {
	return f.RecirculateTo(targetPhase)
}

func (p *Pipeline) CancelFeature(f *feature.Feature) {
	f.Cancel()
}

func (p *Pipeline) CompleteFeature(f *feature.Feature) {
	f.MarkDone()
}

func (p *Pipeline) SaveFeature(f *feature.Feature) error {
	return p.specProvider.SaveFeatureState(f)
}
