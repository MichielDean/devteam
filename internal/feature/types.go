package feature

type Phase string

const (
	PhaseInception   Phase = "inception"
	PhasePlanning    Phase = "planning"
	PhaseConstruction Phase = "construction"
	PhaseReview      Phase = "review"
	PhaseTesting     Phase = "testing"
	PhaseDelivery    Phase = "delivery"
)

func AllPhases() []Phase {
	return []Phase{PhaseInception, PhasePlanning, PhaseConstruction, PhaseReview, PhaseTesting, PhaseDelivery}
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
	StatusDraft       Status = "draft"
	StatusInProgress   Status = "in_progress"
	StatusGateBlocked Status = "gate_blocked"
	StatusPassed      Status = "passed"
	StatusFailed      Status = "failed"
	StatusDone         Status = "done"
	StatusRecirculated Status = "recirculated"
	StatusCancelled    Status = "cancelled"
)

func (s Status) String() string {
	return string(s)
}

type IntakePath string

const (
	IntakeLooseIdea   IntakePath = "loose_idea"
	IntakeExternalSpec IntakePath = "external_spec"
)

func (i IntakePath) String() string {
	return string(i)
}

type ArtifactType string

const (
	ArtifactSpecMD         ArtifactType = "spec_md"
	ArtifactAcceptanceMD   ArtifactType = "acceptance_md"
	ArtifactReposYAML      ArtifactType = "repos_yaml"
	ArtifactPlanMD         ArtifactType = "plan_md"
	ArtifactTasksMD        ArtifactType = "tasks_md"
	ArtifactReviewReport   ArtifactType = "review_report"
	ArtifactTestReport     ArtifactType = "test_report"
	ArtifactDocs           ArtifactType = "docs"
	ArtifactDataModelMD    ArtifactType = "data_model_md"
	ArtifactQuickstartMD   ArtifactType = "quickstart_md"
	ArtifactContractsDir   ArtifactType = "contracts_dir"
)

func (a ArtifactType) String() string {
	return string(a)
}

func ParseArtifactType(s string) ArtifactType {
	switch s {
	case "spec_md":
		return ArtifactSpecMD
	case "acceptance_md":
		return ArtifactAcceptanceMD
	case "repos_yaml":
		return ArtifactReposYAML
	case "plan_md":
		return ArtifactPlanMD
	case "tasks_md":
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

type RoleName string

const (
	RolePM         RoleName = "pm"
	RoleArchitect  RoleName = "architect"
	RoleDeveloper  RoleName = "developer"
	RoleReviewer   RoleName = "reviewer"
	RoleTester     RoleName = "tester"
	RoleOps        RoleName = "ops"
)

func AllRoles() []RoleName {
	return []RoleName{RolePM, RoleArchitect, RoleDeveloper, RoleReviewer, RoleTester, RoleOps}
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