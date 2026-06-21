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
			"spec.md contains functional requirements traced to user stories",
			"spec.md contains error scenarios with specific HTTP status codes",
			"spec.md contains empty state behavior for collections",
			"spec.md contains assumptions marked with [ASSUMPTION:]",
			"spec.md contains constraint register with source references",
			"acceptance.md contains at least one verifiable criterion per user story",
			"acceptance.md criteria follow Given/When/Then format with test level",
			"acceptance.md contains constraint-derived criteria referencing CON- IDs",
			"repos.yaml identifies at least one affected repository",
		},
	},
	{
		Phase:        PhasePlanning,
		GateName:     GatePlanApproved,
		RequiredArts: []ArtifactType{ArtifactPlanMD, ArtifactTasksMD},
		ValidationDescs: []string{
			"plan.md addresses all acceptance criteria from acceptance.md",
			"plan.md includes component design with responsibilities and interfaces",
			"plan.md includes data model with entities, relationships, and state transitions",
			"plan.md includes API contracts with request/response schemas and error responses",
			"plan.md includes test strategy section with testing levels for each component",
			"plan.md includes agent failure mode checks for AI-generated code",
			"plan.md includes constraint verification map tracing constraints to design decisions",
			"plan.md includes cross-component consistency matrix for shared values",
			"tasks.md contains specific file paths for implementation",
			"tasks.md includes done conditions with specific verifiable assertions",
			"tasks.md includes test level required for each task",
			"tasks.md includes constraint references (CON- IDs) for constrained tasks",
			"dependencies between tasks are explicit",
		},
	},
	{
		Phase:        PhaseConstruction,
		GateName:     GateTasksComplete,
		RequiredArts: []ArtifactType{},
		ValidationDescs: []string{
			"code compiles and runs without panicking",
			"tests compile without errors",
		},
	},
	{
		Phase:        PhaseReview,
		GateName:     GateCriteriaMet,
		RequiredArts: []ArtifactType{ArtifactReviewReport},
		ValidationDescs: []string{
			"every acceptance criterion has been reviewed with quoted evidence",
			"every constraint in the register has been reviewed with execution path trace",
			"no critical findings remain unresolved",
			"security review complete for priority-1 features",
			"null pointer safety verified",
			"error paths verified (400, 404, 409, empty state)",
			"over-engineering check completed",
			"missing implementation check completed",
			"cross-component consistency verified across all producers and consumers",
			"negative test vectors verified against implementation",
		},
	},
	{
		Phase:        PhaseTesting,
		GateName:     GateTestsPass,
		RequiredArts: []ArtifactType{ArtifactTestReport},
		ValidationDescs: []string{
			"every acceptance criterion has at least one test",
			"every constraint in the register has at least one test that would fail if violated",
			"conformance tests verify negative test vectors from the constraint register",
			"smoke tests verify the service starts and responds without panics",
			"integration tests exercise full HTTP request/response cycles",
			"JSON shapes match contract ([] not null)",
			"spec-implementation drift checked",
			"no nil pointer panics or null-vs-empty-array mismatches",
			"failed tests have reproduction steps",
			"go test suite passes",
			"multi-component constraints tested across all components",
		},
	},
	{
		Phase:        PhaseDelivery,
		GateName:     GateDocsMatchSpec,
		RequiredArts: []ArtifactType{ArtifactDocs},
		ValidationDescs: []string{
			"documentation exists for every user story",
			"documentation uses spec terminology (not code-internal names)",
			"API documentation covers every endpoint with request/response schemas",
			"changelog references the spec number",
			"cross-repo release order is documented",
			"service starts and responds to HTTP requests",
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
