package feature

type Status string

const (
	StatusDraft           Status = "draft"
	StatusInProgress      Status = "in_progress"
	StatusGateBlocked     Status = "gate_blocked"
	StatusPassed          Status = "passed"
	StatusFailed          Status = "failed"
	StatusDone            Status = "done"
	StatusRecirculated    Status = "recirculated"
	StatusCancelled       Status = "cancelled"
	StatusWaitingFeedback Status = "waiting_for_feedback"
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
	ArtifactInputMD      ArtifactType = "input_md"
	ArtifactSpecMD       ArtifactType = "spec_md"
	ArtifactAcceptanceMD ArtifactType = "acceptance_md"
	ArtifactReposYAML    ArtifactType = "repos_yaml"
	ArtifactPlanMD       ArtifactType = "plan_md"
	ArtifactTasksMD      ArtifactType = "tasks_md"
	ArtifactReviewReport ArtifactType = "review_report"
	ArtifactTestReport   ArtifactType = "test_report"
	ArtifactDocs         ArtifactType = "docs"
	ArtifactDataModelMD  ArtifactType = "data_model_md"
	ArtifactResearchMD   ArtifactType = "research_md"
	ArtifactQuickstartMD ArtifactType = "quickstart_md"
	ArtifactContractsDir ArtifactType = "contracts_dir"
	ArtifactAuditMD      ArtifactType = "audit_md"
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

// canonicalArtifactAliases maps short API path segments to canonical ArtifactTypes.
// Used for the well-known intake/construction artifacts that have a stable DB key.
var canonicalArtifactAliases = map[string]ArtifactType{
	"input":         ArtifactInputMD,
	"spec":          ArtifactSpecMD,
	"acceptance":    ArtifactAcceptanceMD,
	"repos":         ArtifactReposYAML,
	"plan":          ArtifactPlanMD,
	"tasks":         ArtifactTasksMD,
	"review_report": ArtifactReviewReport,
	"test_report":   ArtifactTestReport,
	"docs":          ArtifactDocs,
	"data_model":    ArtifactDataModelMD,
	"research":      ArtifactResearchMD,
	"quickstart":    ArtifactQuickstartMD,
	"contracts":     ArtifactContractsDir,
}

// ArtifactAPIPathToType resolves an API path segment to an ArtifactType.
//
// Canonical aliases (e.g. "spec" -> ArtifactSpecMD) map to their DB key.
// Any other non-empty string is accepted as a pass-through ArtifactType so
// stage-specific artifacts (intent-statement, stakeholder-map, scope-definition,
// etc. — see internal/stage/definitions.go KeyArtifacts) can be submitted and
// retrieved without requiring a code change for every new stage artifact.
// The DB schema stores artifact_type as an arbitrary TEXT column, so this
// pass-through is safe. The empty string is rejected.
func ArtifactAPIPathToType(apiPath string) (ArtifactType, bool) {
	if apiPath == "" {
		return "", false
	}
	if t, ok := canonicalArtifactAliases[apiPath]; ok {
		return t, true
	}
	return ArtifactType(apiPath), true
}

func IsValidPriority(p int) bool {
	return p >= 1 && p <= 3
}