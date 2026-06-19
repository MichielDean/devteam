package feature

import (
	"strings"
)

func ValidateTransition(from, to Phase) bool {
	phases := AllPhases()
	fromIdx := -1
	toIdx := -1
	for i, p := range phases {
		if p == from {
			fromIdx = i
		}
		if p == to {
			toIdx = i
		}
	}
	if fromIdx < 0 || toIdx < 0 {
		return false
	}
	return toIdx == fromIdx+1
}

func RecirculationTarget(from Phase, reason string) Phase {
	switch from {
	case PhaseReview:
		if strings.Contains(reason, "architect") || strings.Contains(reason, "plan") {
			return PhasePlanning
		}
		return PhaseConstruction
	case PhaseTesting:
		return PhaseConstruction
	case PhaseDelivery:
		return PhaseTesting
	case PhaseInception:
		return PhaseInception
	default:
		return PhaseConstruction
	}
}

type GateDefinition struct {
	Phase           Phase
	GateName        GateName
	RequiredArts    []ArtifactType
	ValidationDescs []string
}

var GateDefinitions = []GateDefinition{
	{
		Phase:        PhaseInception,
		GateName:     GateSpecApproved,
		RequiredArts: []ArtifactType{ArtifactSpecMD, ArtifactAcceptanceMD, ArtifactReposYAML},
		ValidationDescs: []string{
			"spec.md contains at least one user story with priority",
			"acceptance.md contains at least one verifiable criterion per story",
			"repos.yaml identifies at least one affected repository",
		},
	},
	{
		Phase:        PhasePlanning,
		GateName:     GatePlanApproved,
		RequiredArts: []ArtifactType{ArtifactPlanMD, ArtifactTasksMD},
		ValidationDescs: []string{
			"plan.md addresses all acceptance criteria from acceptance.md",
			"tasks.md contains specific file paths for implementation",
			"dependencies between tasks are explicit",
		},
	},
	{
		Phase:        PhaseConstruction,
		GateName:     GateTasksComplete,
		RequiredArts: []ArtifactType{},
		ValidationDescs: []string{
			"code compiles in every affected repository",
			"no placeholder or stub code remains",
			"each repository's changes are independently buildable",
		},
	},
	{
		Phase:        PhaseReview,
		GateName:     GateCriteriaMet,
		RequiredArts: []ArtifactType{ArtifactReviewReport},
		ValidationDescs: []string{
			"every acceptance criterion has been reviewed with evidence",
			"no critical findings remain unresolved",
			"security review complete for priority-1 features",
		},
	},
	{
		Phase:        PhaseTesting,
		GateName:     GateTestsPass,
		RequiredArts: []ArtifactType{ArtifactTestReport},
		ValidationDescs: []string{
			"every acceptance criterion has at least one test",
			"all critical-path tests pass",
			"failed tests have reproduction steps",
		},
	},
	{
		Phase:        PhaseDelivery,
		GateName:     GateDocsMatchSpec,
		RequiredArts: []ArtifactType{ArtifactDocs},
		ValidationDescs: []string{
			"documentation uses spec terminology",
			"changelog references the spec number",
			"cross-repo release order is documented",
		},
	},
}

func GetGateDefinition(phase Phase) *GateDefinition {
	for i := range GateDefinitions {
		if GateDefinitions[i].Phase == phase {
			return &GateDefinitions[i]
		}
	}
	return nil
}

func RequiredArtifactsForPhase(phase Phase) []ArtifactType {
	gd := GetGateDefinition(phase)
	if gd == nil {
		return nil
	}
	return gd.RequiredArts
}
