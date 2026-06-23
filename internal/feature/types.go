package feature

type Phase string

const (
	PhaseInception    Phase = "inception"
	PhasePlanning     Phase = "planning"
	PhaseConstruction Phase = "construction"
	PhaseReview       Phase = "review"
	PhaseTesting      Phase = "testing"
	PhaseDelivery     Phase = "delivery"
)

func AllPhases() []Phase {
	return []Phase{PhaseInception, PhasePlanning, PhaseConstruction, PhaseReview, PhaseTesting, PhaseDelivery}
}

// NextPhase returns the phase after the given one, or "" if it's the last phase.
func NextPhase(p Phase) Phase {
	phases := AllPhases()
	for i, phase := range phases {
		if phase == p && i+1 < len(phases) {
			return phases[i+1]
		}
	}
	return ""
}

func (p Phase) String() string {
	return string(p)
}

func ParsePhase(s string) Phase {
	switch s {
	case "inception":
		return PhaseInception
	case "planning":
		return PhasePlanning
	case "construction":
		return PhaseConstruction
	case "review":
		return PhaseReview
	case "testing":
		return PhaseTesting
	case "delivery":
		return PhaseDelivery
	default:
		return PhaseInception
	}
}

type Status string

const (
	StatusDraft         Status = "draft"
	StatusInProgress    Status = "in_progress"
	StatusGateBlocked   Status = "gate_blocked"
	StatusPassed        Status = "passed"
	StatusFailed        Status = "failed"
	StatusDone          Status = "done"
	StatusRecirculated  Status = "recirculated"
	StatusCancelled     Status = "cancelled"
	StatusWaitingHuman  Status = "waiting_for_human"
)

func (s Status) String() string {
	return string(s)
}

type IntakePath string

const (
	IntakeLooseIdea    IntakePath = "loose_idea"
	IntakeExternalSpec IntakePath = "external_spec"
)

func (i IntakePath) String() string {
	return string(i)
}

type ArtifactType string

const (
	ArtifactInputMD       ArtifactType = "input_md"
	ArtifactSpecMD        ArtifactType = "spec_md"
	ArtifactAcceptanceMD  ArtifactType = "acceptance_md"
	ArtifactReposYAML     ArtifactType = "repos_yaml"
	ArtifactPlanMD        ArtifactType = "plan_md"
	ArtifactTasksMD       ArtifactType = "tasks_md"
	ArtifactReviewReport  ArtifactType = "review_report"
	ArtifactTestReport    ArtifactType = "test_report"
	ArtifactDocs          ArtifactType = "docs"
	ArtifactDataModelMD   ArtifactType = "data_model_md"
	ArtifactResearchMD    ArtifactType = "research_md"
	ArtifactQuickstartMD  ArtifactType = "quickstart_md"
	ArtifactContractsDir  ArtifactType = "contracts_dir"
	ArtifactAuditMD       ArtifactType = "audit_md"
)

func (a ArtifactType) String() string {
	return string(a)
}

func ParseArtifactType(s string) ArtifactType {
	switch s {
	case "input_md", "input":
		return ArtifactInputMD
	case "spec_md", "spec":
		return ArtifactSpecMD
	case "acceptance_md", "acceptance":
		return ArtifactAcceptanceMD
	case "repos_yaml", "repos":
		return ArtifactReposYAML
	case "plan_md", "plan":
		return ArtifactPlanMD
	case "tasks_md", "tasks":
		return ArtifactTasksMD
	case "review_report":
		return ArtifactReviewReport
	case "test_report":
		return ArtifactTestReport
	case "docs":
		return ArtifactDocs
	case "data_model_md":
		return ArtifactDataModelMD
	case "quickstart_md":
		return ArtifactQuickstartMD
	case "contracts_dir":
		return ArtifactContractsDir
	default:
		return ArtifactSpecMD
	}
}

// ArtifactAPIPathToType maps API path parameter values to ArtifactType.
// This is the reverse mapping for URL parameters like /api/features/:id/artifacts/spec
func ArtifactAPIPathToType(apiPath string) (ArtifactType, bool) {
	m := map[string]ArtifactType{
		"input":          ArtifactInputMD,
		"spec":           ArtifactSpecMD,
		"acceptance":     ArtifactAcceptanceMD,
		"repos":          ArtifactReposYAML,
		"plan":           ArtifactPlanMD,
		"tasks":          ArtifactTasksMD,
		"review_report":  ArtifactReviewReport,
		"test_report":    ArtifactTestReport,
		"docs":           ArtifactDocs,
		"data_model":     ArtifactDataModelMD,
		"quickstart":     ArtifactQuickstartMD,
		"contracts":      ArtifactContractsDir,
	}
	t, ok := m[apiPath]
	return t, ok
}

type RoleName string

const (
	RolePM        RoleName = "pm"
	RoleArchitect RoleName = "architect"
	RoleDeveloper RoleName = "developer"
	RoleReviewer  RoleName = "reviewer"
	RoleTester    RoleName = "tester"
	RoleOps       RoleName = "ops"
)

func AllRoles() []RoleName {
	return []RoleName{RolePM, RoleArchitect, RoleDeveloper, RoleReviewer, RoleTester, RoleOps}
}

func ValidPhaseNames() []string {
	phases := AllPhases()
	names := make([]string, len(phases))
	for i, p := range phases {
		names[i] = string(p)
	}
	return names
}

func IsValidPhase(name string) bool {
	for _, p := range AllPhases() {
		if string(p) == name {
			return true
		}
	}
	return false
}

// IsValidPriority returns true if the priority is 1, 2, or 3
func IsValidPriority(p int) bool {
	return p >= 1 && p <= 3
}

func (r RoleName) String() string {
	return string(r)
}

type GateName string

const (
	GateSpecApproved  GateName = "spec_approved"
	GatePlanApproved  GateName = "plan_approved"
	GateTasksComplete GateName = "tasks_complete"
	GateCriteriaMet   GateName = "criteria_met"
	GateTestsPass     GateName = "tests_pass"
	GateDocsMatchSpec GateName = "docs_match_spec"
)

func (g GateName) String() string {
	return string(g)
}
