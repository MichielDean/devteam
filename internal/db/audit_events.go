package db

// Audit event types (AIDLC v2 68-event audit trail, adapted for 10-agent platform).
// Categories: Workflow Lifecycle, Phase Lifecycle, Stage Lifecycle, Session,
// Initialization, Navigation, Interaction, Artifact, Subagent, Utility,
// Error/Recovery, Construction Bolt, Worktree, Practices, Merge Dispatch,
// Sensors, Learning Loop, Swarm.
const (
	// Workflow Lifecycle
	AuditWorkflowStart          = "WORKFLOW_START"
	AuditWorkflowComplete       = "WORKFLOW_COMPLETE"
	AuditWorkflowReset          = "WORKFLOW_RESET"
	AuditWorkflowResume         = "WORKFLOW_RESUME"

	// Phase Lifecycle
	AuditPhaseStart             = "PHASE_START"
	AuditPhaseComplete          = "PHASE_COMPLETE"
	AuditPhaseSkipped           = "PHASE_SKIPPED"
	AuditPhaseJumped            = "PHASE_JUMPED"

	// Stage Lifecycle
	AuditStageStart             = "STAGE_START"
	AuditStageAwaitingApproval  = "STAGE_AWAITING_APPROVAL"
	AuditStageRevising          = "STAGE_REVISING"
	AuditStageCompleted         = "STAGE_COMPLETED"
	AuditStageSkipped           = "STAGE_SKIPPED"
	AuditStageJumped            = "STAGE_JUMPED"
	AuditStageAdvanced          = "STAGE_ADVANCED"

	// Session
	AuditSessionStart           = "SESSION_START"
	AuditSessionEnd             = "SESSION_END"
	AuditSessionResume          = "SESSION_RESUME"
	AuditSessionTimeout         = "SESSION_TIMEOUT"

	// Initialization
	AuditWorkspaceScaffold      = "WORKSPACE_SCAFFOLD"
	AuditWorkspaceDetect        = "WORKSPACE_DETECT"
	AuditStateInit              = "STATE_INIT"

	// Navigation
	AuditJumpToStage            = "JUMP_TO_STAGE"
	AuditJumpToPhase            = "JUMP_TO_PHASE"
	AuditScopeChange            = "SCOPE_CHANGE"
	AuditDepthChange            = "DEPTH_CHANGE"

	// Interaction
	AuditGateApproved           = "GATE_APPROVED"
	AuditGateRejected           = "GATE_REJECTED"
	AuditGateAcceptAsIs         = "GATE_ACCEPT_AS_IS"
	AuditQuestionAsked          = "QUESTION_ASKED"
	AuditQuestionAnswered       = "QUESTION_ANSWERED"

	// Artifact
	AuditArtifactCreated        = "ARTIFACT_CREATED"
	AuditArtifactUpdated        = "ARTIFACT_UPDATED"
	AuditArtifactDeleted        = "ARTIFACT_DELETED"

	// Subagent
	AuditSubagentCompleted      = "SUBAGENT_COMPLETED"

	// Utility
	AuditUtilityRun             = "UTILITY_RUN"

	// Error/Recovery
	AuditErrorStage             = "ERROR_STAGE"
	AuditHaltAndAsk             = "HALT_AND_ASK"

	// Construction Bolt
	AuditBoltStarted            = "BOLT_STARTED"
	AuditBoltCompleted          = "BOLT_COMPLETED"
	AuditBoltFailed             = "BOLT_FAILED"
	AuditLadderPrompt           = "LADDER_PROMPT"

	// Worktree
	AuditWorktreeCreate         = "WORKTREE_CREATE"
	AuditWorktreeMerge          = "WORKTREE_MERGE"
	AuditWorktreeDiscard        = "WORKTREE_DISCARD"
	AuditWorktreeConflict       = "WORKTREE_CONFLICT"
	AuditWorktreeVerify         = "WORKTREE_VERIFY"
	AuditWorktreeCleanup        = "WORKTREE_CLEANUP"
	AuditWorktreeRetry          = "WORKTREE_RETRY"

	// Practices
	AuditPracticesDiscovered    = "PRACTICES_DISCOVERED"
	AuditPracticesApplied       = "PRACTICES_APPLIED"
	AuditPracticesViolated      = "PRACTICES_VIOLATED"
	AuditPracticesUpdated       = "PRACTICES_UPDATED"

	// Merge Dispatch
	AuditMergeStarted           = "MERGE_STARTED"
	AuditMergeCompleted         = "MERGE_COMPLETED"
	AuditMergeConflict          = "MERGE_CONFLICT"

	// Sensors
	AuditSensorTriggered        = "SENSOR_TRIGGERED"
	AuditSensorAlert            = "SENSOR_ALERT"
	AuditSensorResolved         = "SENSOR_RESOLVED"
	AuditSensorBaseline         = "SENSOR_BASELINE"
	AuditSensorDrift            = "SENSOR_DRIFT"

	// Learning Loop
	AuditRuleLearned            = "RULE_LEARNED"
	AuditRuleApplied            = "RULE_APPLIED"
	AuditRuleInvalidated        = "RULE_INVALIDATED"

	// Swarm
	AuditSwarmSpawn             = "SWARM_SPAWN"
	AuditSwarmComplete          = "SWARM_COMPLETE"
	AuditSwarmFailed            = "SWARM_FAILED"
	AuditSwarmRebalance         = "SWARM_REBALANCE"
	AuditSwarmExpand            = "SWARM_EXPAND"
	AuditSwarmContract          = "SWARM_CONTRACT"
)