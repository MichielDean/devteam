package init

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `# Dev Team Configuration

version: "1.0"

pipeline:
  phases:
    - name: inception
      roles: [pm, architect]
      gate: spec_approved
      artifacts: [spec.md, acceptance.md, repos.yaml]
      rules: rules/pipeline/core-workflow.md

    - name: planning
      roles: [architect]
      gate: plan_approved
      artifacts: [plan.md, tasks.md]
      rules: rules/pipeline/planning/

    - name: construction
      roles: [developer]
      gate: tasks_complete
      artifacts: [implementation across repos]
      rules: rules/pipeline/construction/

    - name: review
      roles: [reviewer]
      gate: criteria_met
      artifacts: [review report]
      rules: rules/pipeline/review/

    - name: testing
      roles: [tester]
      gate: tests_pass
      artifacts: [test report]
      rules: rules/pipeline/testing/

    - name: delivery
      roles: [ops]
      gate: docs_match_spec
      artifacts: [docs, release]
      rules: rules/pipeline/delivery/

  roles:
    pm:
      name: Product Manager
      description: Owns the what and why. Explores, clarifies, refines ideas into specs.
      instructions: roles/pm/INSTRUCTIONS.md
      phase_rules: rules/pipeline/inception/

    architect:
      name: Architect
      description: Owns the how. Creates technical plans, designs cross-repo boundaries.
      instructions: roles/architect/INSTRUCTIONS.md
      phase_rules: rules/pipeline/planning/

    developer:
      name: Developer
      description: Writes code across repos. Follows spec, plan, and task breakdown.
      instructions: roles/developer/INSTRUCTIONS.md
      phase_rules: rules/pipeline/construction/

    reviewer:
      name: Code Reviewer
      description: Adversarial review against spec acceptance criteria.
      instructions: roles/reviewer/INSTRUCTIONS.md
      phase_rules: rules/pipeline/review/

    tester:
      name: Tester
      description: Writes and runs tests from user stories. Traces tests to spec requirements.
      instructions: roles/tester/INSTRUCTIONS.md
      phase_rules: rules/pipeline/testing/

    ops:
      name: Release Engineer
      description: Owns deployment, docs, and cross-repo coordination.
      instructions: roles/ops/INSTRUCTIONS.md
      phase_rules: rules/pipeline/delivery/

  extensions:
    security:
      opt_in: true
      load_for_priority: [1, 2]
      rules: rules/pipeline/extensions/security/rules.md

    resiliency:
      opt_in: true
      load_for_priority: [1]
      rules: rules/pipeline/extensions/resiliency/rules.md

    error-recovery:
      opt_in: false
      always_on: true
      rules: rules/pipeline/extensions/error-recovery/rules.md

    overconfidence-prevention:
      opt_in: false
      always_on: true
      rules: rules/pipeline/extensions/overconfidence-prevention/rules.md

intake:
  loose_idea:
    description: Rough idea, sentence, or vague description. PM explores and refines into structured spec.
    output: [spec.md, acceptance.md, repos.yaml]

  external_spec:
    description: PRD, RFC, roadmap, Jira epic, or other formal requirements document.
    output: [N x spec.md, acceptance.md, repos.yaml]

spec_repo:
  path: .
  specs_dir: specs/
  constitution_dir: constitution/
`

const defaultRepos = `# Repository Registry

# Maps feature specs to the implementation repos they touch.
# Add your project repositories here.

repos: []
`

type Initializer struct {
	baseDir string
}

func NewInitializer(baseDir string) *Initializer {
	return &Initializer{baseDir: baseDir}
}

func (init *Initializer) Init() error {
	dirs := []string{
		filepath.Join(init.baseDir, "specs"),
		filepath.Join(init.baseDir, "roles", "pm"),
		filepath.Join(init.baseDir, "roles", "architect"),
		filepath.Join(init.baseDir, "roles", "developer"),
		filepath.Join(init.baseDir, "roles", "reviewer"),
		filepath.Join(init.baseDir, "roles", "tester"),
		filepath.Join(init.baseDir, "roles", "ops"),
		filepath.Join(init.baseDir, "rules", "pipeline"),
		filepath.Join(init.baseDir, "rules", "pipeline", "inception"),
		filepath.Join(init.baseDir, "rules", "pipeline", "planning"),
		filepath.Join(init.baseDir, "rules", "pipeline", "construction"),
		filepath.Join(init.baseDir, "rules", "pipeline", "review"),
		filepath.Join(init.baseDir, "rules", "pipeline", "testing"),
		filepath.Join(init.baseDir, "rules", "pipeline", "delivery"),
		filepath.Join(init.baseDir, "rules", "pipeline", "extensions", "security"),
		filepath.Join(init.baseDir, "rules", "pipeline", "extensions", "resiliency"),
		filepath.Join(init.baseDir, "rules", "pipeline", "extensions", "error-recovery"),
		filepath.Join(init.baseDir, "rules", "pipeline", "extensions", "overconfidence-prevention"),
		filepath.Join(init.baseDir, "constitution"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	configPath := filepath.Join(init.baseDir, "devteam.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("writing devteam.yaml: %w", err)
		}
		fmt.Printf("Created %s\n", configPath)
	} else {
		fmt.Printf("Already exists: %s\n", configPath)
	}

	reposPath := filepath.Join(init.baseDir, "repos.yaml")
	if _, err := os.Stat(reposPath); os.IsNotExist(err) {
		if err := os.WriteFile(reposPath, []byte(defaultRepos), 0644); err != nil {
			return fmt.Errorf("writing repos.yaml: %w", err)
		}
		fmt.Printf("Created %s\n", reposPath)
	} else {
		fmt.Printf("Already exists: %s\n", reposPath)
	}

	roleInstructions := map[string]string{
		"pm": `# Product Manager

You are the Product Manager role in the Dev Team pipeline.

## Responsibilities

- Own the **what** and **why** of the feature
- Explore, clarify, and refine loose ideas into structured specifications
- Decompose external roadmaps into independent feature specs with dependency edges
- Write spec.md with user stories and acceptance criteria
- Identify affected repositories and create repos.yaml
- Flag gaps, contradictions, and ambiguities in requirements

## Quality Gate

Before passing the inception gate, you MUST produce:

1. **spec.md** — Feature specification with user stories, each having priority and independent test
2. **acceptance.md** — Verifiable acceptance criteria traced to user stories
3. **repos.yaml** — Repository registry identifying all affected repositories

## Rules

- Every user story MUST have an independent test
- Acceptance criteria MUST be verifiable (not vague)
- repos.yaml MUST identify at least one repository
- Flag gaps rather than assuming missing details
`,
		"architect": `# Architect

You are the Architect role in the Dev Team pipeline.

## Responsibilities

- Own the **how** of the feature
- Create technical plans from approved specs
- Design cross-repo boundaries and interfaces
- Validate spec feasibility before implementation
- Produce plan.md and tasks.md with specific file paths

## Quality Gate

Before passing the planning gate, you MUST produce:

1. **plan.md** — Technical implementation plan addressing all acceptance criteria
2. **tasks.md** — Task breakdown with specific file paths and explicit dependencies

## Rules

- plan.md MUST address all acceptance criteria from acceptance.md
- tasks.md MUST contain specific file paths for implementation
- Dependencies between tasks MUST be explicit
- Cross-repo boundaries MUST be documented
`,
		"developer": `# Developer

You are the Developer role in the Dev Team pipeline.

## Responsibilities

- Write code across all repositories declared in repos.yaml
- Follow the spec, plan, and task breakdown exactly
- Produce implementation that compiles and is independently buildable
- No placeholder or stub code — every function must work

## Quality Gate

Before passing the construction gate, you MUST ensure:

1. Code compiles in every affected repository
2. No placeholder or stub code remains
3. Each repository's changes are independently buildable

## Rules

- Follow the plan and tasks exactly — no scope creep
- Every function must have a real implementation, not a stub
- Commit messages MUST reference the spec number
- Changes across repos MUST be coordinated with consistent commit messages
`,
		"reviewer": `# Code Reviewer

You are the Code Reviewer role in the Dev Team pipeline.

## Responsibilities

- Adversarial review against spec acceptance criteria
- Verify the **right thing** was built, not just that something was built
- Produce review report with evidence-quoted findings
- Security review for priority-1 features
- Trace every acceptance criterion to implementation evidence

## Quality Gate

Before passing the review gate, you MUST produce:

1. **review-report.md** — Review report with evidence for each acceptance criterion

## Rules

- Every acceptance criterion MUST be reviewed with evidence
- No critical findings may remain unresolved
- Security review MUST be complete for priority-1 features
- Findings MUST quote specific code or spec text
`,
		"tester": `# Tester

You are the Tester role in the Dev Team pipeline.

## Responsibilities

- Write and run tests from user stories
- Trace tests to spec requirements
- Produce test report with traced test IDs
- Failed tests MUST have reproduction steps

## Quality Gate

Before passing the testing gate, you MUST produce:

1. **test-report.md** — Test report with traced test IDs mapping to acceptance criteria

## Rules

- Every acceptance criterion MUST have at least one test
- All critical-path tests MUST pass
- Failed tests MUST have reproduction steps
- Test IDs MUST trace to acceptance criteria IDs
`,
		"ops": `# Release Engineer

You are the Release Engineer (Ops) role in the Dev Team pipeline.

## Responsibilities

- Own deployment, documentation, and cross-repo coordination
- Ensure documentation uses spec terminology
- Coordinate release order across repositories
- Produce release notes and changelog

## Quality Gate

Before passing the delivery gate, you MUST ensure:

1. Documentation uses spec terminology consistently
2. Changelog references the spec number
3. Cross-repo release order is documented

## Rules

- Documentation MUST use the same terminology as spec.md
- Changelog MUST reference the spec number
- Cross-repo release order MUST be documented
- No undocumented breaking changes
`,
	}

	for roleName, instructions := range roleInstructions {
		instructionsPath := filepath.Join(init.baseDir, "roles", roleName, "INSTRUCTIONS.md")
		if _, err := os.Stat(instructionsPath); os.IsNotExist(err) {
			if err := os.WriteFile(instructionsPath, []byte(instructions), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", instructionsPath, err)
			}
			fmt.Printf("Created %s\n", instructionsPath)
		} else {
			fmt.Printf("Already exists: %s\n", instructionsPath)
		}
	}

	coreWorkflow := filepath.Join(init.baseDir, "rules", "pipeline", "core-workflow.md")
	if _, err := os.Stat(coreWorkflow); os.IsNotExist(err) {
		content := "# Dev Team Pipeline Governance\n\nThis directory should contain the Dev Team pipeline rules.\nSee https://github.com/MichielDean/devteam for the full rule set.\n"
		if err := os.WriteFile(coreWorkflow, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing core-workflow.md: %w", err)
		}
		fmt.Printf("Created %s\n", coreWorkflow)
	}

	constitutionPath := filepath.Join(init.baseDir, "constitution", "constitution.md")
	if _, err := os.Stat(constitutionPath); os.IsNotExist(err) {
		content := "# Constitution\n\nAdd your project's governing principles here.\n"
		if err := os.WriteFile(constitutionPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing constitution.md: %w", err)
		}
		fmt.Printf("Created %s\n", constitutionPath)
	}

	fmt.Println("\nDev Team project initialized successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit devteam.yaml to customize your pipeline")
	fmt.Println("  2. Edit repos.yaml to add your repositories")
	fmt.Println("  3. Edit roles/*/INSTRUCTIONS.md for your team")
	fmt.Println("  4. Run: devteam intake --type loose --text \"your feature idea\"")

	return nil
}