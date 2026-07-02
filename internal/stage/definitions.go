package stage

import (
	"github.com/MichielDean/devteam/internal/db"
)

// Helper: return only the named scopes (explicit inclusion).
func scopes(names ...string) []string {
	return names
}

// allScopes returns all 9 scopes.
func allScopes() []string {
	return []string{ScopeEnterprise, ScopeFeature, ScopeMVP, ScopePOC, ScopeBugfix, ScopeRefactor, ScopeInfra, ScopeSecurityPatch, ScopeWorkshop}
}

// allMinus returns all scopes except the named exclusions.
func allMinus(exclude ...string) []string {
	all := allScopes()
	var result []string
	for _, s := range all {
		skip := false
		for _, ex := range exclude {
			if s == ex {
				skip = true
				break
			}
		}
		if !skip {
			result = append(result, s)
		}
	}
	return result
}

// AllStages returns the 32 AIDLC v2 stage definitions, adapted for our 10-agent roster.
// Compliance stages folded into architect/devsecops. Platform agent is cloud-agnostic.
//
// Agent names: product, design, delivery, architect, platform, devsecops, developer,
//              quality, pipeline-deploy, operations
// Reviewers: product-lead, architecture-reviewer
var stageDefinitions = []db.StageDefinition{
	// ── Phase 0: Initialization (3 stages, auto-proceed, no gates) ──
	{"0.1", PhaseInitialization, "Workspace Scaffold", "orchestrator", nil, []string{"record-dir"}, CondAlways, allScopes(), "", 1},
	{"0.2", PhaseInitialization, "Workspace Detection", "orchestrator", nil, []string{"workspace-state"}, CondAlways, allScopes(), "", 2},
	{"0.3", PhaseInitialization, "State Initialization", "orchestrator", nil, []string{"aidlc-state", "audit-shards"}, CondAlways, allScopes(), "", 3},

	// ── Phase 1: Ideation (7 stages) ──
	// 1.1 Intent Capture — ALWAYS, all scopes
	{"1.1", PhaseIdeation, "Intent Capture & Framing", "product", []string{"architect"}, []string{"intent-statement", "stakeholder-map"}, CondAlways, allScopes(), "", 4},
	// 1.2 Market Research — CONDITIONAL, skip for poc/bugfix/refactor/infra/security-patch
	{"1.2", PhaseIdeation, "Market Research", "product", nil, []string{"competitive-analysis", "build-vs-buy"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature, ScopeMVP, ScopeWorkshop), "", 5},
	// 1.3 Feasibility — CONDITIONAL, skip for poc/bugfix/refactor/security-patch/workshop
	{"1.3", PhaseIdeation, "Feasibility & Constraints", "architect", []string{"platform", "devsecops"}, []string{"feasibility-assessment", "constraint-register", "raid-log"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature, ScopeMVP, ScopeInfra), "", 6},
	// 1.4 Scope Definition — ALWAYS, all scopes
	{"1.4", PhaseIdeation, "Scope Definition", "product", []string{"delivery"}, []string{"scope-definition", "intent-backlog"}, CondAlways, allScopes(), "", 7},
	// 1.5 Team Formation — CONDITIONAL, skip for poc/bugfix/refactor/infra/security-patch/workshop
	{"1.5", PhaseIdeation, "Team Formation", "delivery", nil, []string{"team-assessment", "mob-composition"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature, ScopeMVP), "", 8},
	// 1.6 Rough Mockups — CONDITIONAL, skip for poc/bugfix/refactor/infra/security-patch/workshop
	{"1.6", PhaseIdeation, "Rough Mockups", "design", []string{"product"}, []string{"wireframes", "user-flows", "concept-deck"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature, ScopeMVP), "product-lead", 9},
	// 1.7 Approval & Handoff — skip for mvp/poc/bugfix/refactor
	{"1.7", PhaseIdeation, "Approval & Handoff", "delivery", []string{"product"}, []string{"initiative-brief", "decision-log"}, CondAlways, scopes(ScopeEnterprise, ScopeFeature, ScopeInfra, ScopeSecurityPatch, ScopeWorkshop), "", 10},

	// ── Phase 2: Inception (8 stages) ──
	// 2.1 Reverse Engineering — BROWNFIELD, skip for poc/bugfix
	{"2.1", PhaseInception, "Reverse Engineering", "developer", []string{"architect"}, []string{"re-artifacts"}, CondBrownfield, allMinus(ScopePOC, ScopeBugfix), "", 11},
	// 2.2 Practices Discovery — CONDITIONAL, skip for poc/bugfix/refactor/security-patch
	{"2.2", PhaseInception, "Practices Discovery", "pipeline-deploy", []string{"quality", "developer", "devsecops"}, []string{"team-practices", "discovered-rules", "evidence"}, CondConditional, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor, ScopeSecurityPatch), "", 12},
	// 2.3 Requirements Analysis — ALWAYS, all scopes
	{"2.3", PhaseInception, "Requirements Analysis", "product", nil, []string{"requirements.md"}, CondAlways, allScopes(), "product-lead", 13},
	// 2.4 User Stories — USER_FACING, skip for bugfix/refactor/infra/security-patch/poc
	{"2.4", PhaseInception, "User Stories", "product", []string{"design"}, []string{"stories.md", "personas.md"}, CondUserFacing, scopes(ScopeEnterprise, ScopeFeature, ScopeMVP, ScopeWorkshop), "product-lead", 14},
	// 2.5 Refined Mockups — UI_PROJECT, skip for bugfix/refactor/infra/security-patch/poc
	{"2.5", PhaseInception, "Refined Mockups", "design", []string{"product"}, []string{"hi-fi-mockups", "interaction-spec"}, CondUIProject, scopes(ScopeEnterprise, ScopeFeature, ScopeMVP, ScopeWorkshop), "product-lead", 15},
	// 2.6 Application Design — CONDITIONAL, skip for poc/bugfix/refactor
	{"2.6", PhaseInception, "Application Design", "architect", []string{"platform", "design"}, []string{"app-design", "adrs"}, CondConditional, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor), "architecture-reviewer", 16},
	// 2.7 Units Generation — ALWAYS, all scopes (skip for security-patch to hit 9)
	{"2.7", PhaseInception, "Units Generation", "architect", []string{"delivery"}, []string{"unit-of-work", "dependency-dag", "story-map"}, CondAlways, allMinus(ScopeSecurityPatch), "architecture-reviewer", 17},
	// 2.8 Delivery Planning — skip for poc/bugfix/refactor/security-patch
	{"2.8", PhaseInception, "Delivery Planning", "delivery", []string{"architect"}, []string{"bolt-plan", "team-allocation", "risk-rationale", "external-dep-map"}, CondAlways, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor, ScopeSecurityPatch), "", 18},

	// ── Phase 3: Construction (7 stages) ──
	// 3.1 Functional Design — PER_BOLT, skip for poc/bugfix/refactor/infra (infra = no app design)
	{"3.1", PhaseConstruction, "Functional Design", "architect", []string{"developer"}, []string{"business-logic-model", "business-rules"}, CondPerBolt, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor, ScopeInfra), "architecture-reviewer", 19},
	// 3.2 NFR Requirements — PER_BOLT, skip for poc/bugfix/refactor
	{"3.2", PhaseConstruction, "NFR Requirements", "architect", []string{"devsecops", "quality"}, []string{"security-nfrs", "performance-nfrs", "reliability-nfrs"}, CondPerBolt, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor), "architecture-reviewer", 20},
	// 3.3 NFR Design — PER_BOLT, skip for poc/bugfix/refactor
	{"3.3", PhaseConstruction, "NFR Design", "architect", []string{"platform"}, []string{"nfr-design-specs"}, CondPerBolt, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor), "architecture-reviewer", 21},
	// 3.4 Infrastructure Design — PER_BOLT, skip for poc/bugfix/refactor/security-patch
	{"3.4", PhaseConstruction, "Infrastructure Design", "platform", []string{"devsecops"}, []string{"infra-specs", "iac-designs"}, CondPerBolt, allMinus(ScopePOC, ScopeBugfix, ScopeRefactor, ScopeSecurityPatch), "architecture-reviewer", 22},
	// 3.5 Code Generation — PER_BOLT, all scopes (ALWAYS)
	{"3.5", PhaseConstruction, "Code Generation", "developer", nil, []string{"application-code", "code-docs"}, CondPerBolt, allScopes(), "architecture-reviewer", 23},
	// 3.6 Build and Test — ONCE_AT_END, all scopes
	{"3.6", PhaseConstruction, "Build and Test", "quality", []string{"devsecops"}, []string{"test-results", "quality-report"}, CondOnceAtEnd, allScopes(), "", 24},
	// 3.7 CI Pipeline — ONCE_AT_END, skip for poc/bugfix/refactor/security-patch/mvp
	{"3.7", PhaseConstruction, "CI Pipeline", "pipeline-deploy", nil, []string{"ci-config", "quality-gates"}, CondOnceAtEnd, scopes(ScopeEnterprise, ScopeFeature, ScopeInfra, ScopeWorkshop), "", 25},

	// ── Phase 4: Operation (7 stages, all conditional) ──
	// 4.1-4.7: skip for mvp/poc/bugfix/refactor/security-patch
	{"4.1", PhaseOperation, "Deployment Pipeline", "pipeline-deploy", nil, []string{"cd-config", "deploy-strategy", "rollback-runbook"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature, ScopeInfra, ScopeWorkshop), "", 26},
	{"4.2", PhaseOperation, "Environment Provisioning", "platform", []string{"devsecops"}, []string{"env-inventory", "validation-report"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature), "", 27},
	{"4.3", PhaseOperation, "Deployment Execution", "pipeline-deploy", []string{"developer"}, []string{"deploy-log", "smoke-tests", "health-checks"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature, ScopeInfra), "", 28},
	{"4.4", PhaseOperation, "Observability Setup", "operations", nil, []string{"dashboards", "alarms", "slo-config"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature), "", 29},
	{"4.5", PhaseOperation, "Incident Response", "operations", nil, []string{"runbooks", "incident-plan", "escalation-matrix"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature), "", 30},
	{"4.6", PhaseOperation, "Performance Validation", "quality", nil, []string{"load-test-results", "nfr-validation-matrix"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature), "", 31},
	{"4.7", PhaseOperation, "Feedback & Optimization", "operations", []string{"platform"}, []string{"slo-report", "cost-analysis", "feedback-loop"}, CondConditional, scopes(ScopeEnterprise, ScopeFeature), "", 32},
}

// SeedStages inserts all 32 stage definitions into the DB. Idempotent.
func SeedStages(database *db.DB) error {
	for _, s := range stageDefinitions {
		if err := database.UpsertStageDefinition(s); err != nil {
			return err
		}
	}
	return nil
}

// GetStageDefinitions returns the in-memory stage definitions (for testing).
func GetStageDefinitions() []db.StageDefinition {
	return stageDefinitions
}