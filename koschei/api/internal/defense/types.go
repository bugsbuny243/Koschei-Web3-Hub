package defense

import "time"

const SchemaVersion = "koschei-defense-agent-runtime-v1"

const (
	ModeDisabled = "disabled"
	ModeShadow   = "shadow"
)

const (
	RuntimeDisabled        = "disabled"
	RuntimeObserved        = "observed"
	RuntimeEvidencePending = "evidence_pending"
	RuntimePartial         = "partial"
	RuntimeBlocked         = "blocked"
)

const (
	ToolObserved        = "observed"
	ToolEvidencePending = "evidence_pending"
	ToolNotApplicable   = "not_applicable"
	ToolFailed          = "failed"
)

const (
	RoleProgramArchaeologist = "program_archaeologist"
	RoleStaticAnalyzer       = "static_analyzer"
	RoleReproductionAgent    = "reproduction_agent"
)

type ToolDefinition struct {
	Name              string   `json:"name"`
	Purpose           string   `json:"purpose"`
	ReadOnly          bool     `json:"read_only"`
	NetworkAccess     string   `json:"network_access"`
	AllowedRoles      []string `json:"allowed_roles"`
	ProducesEvidence  bool     `json:"produces_evidence"`
	CanChangeVerdict  bool     `json:"can_change_verdict"`
	RequiresSandbox   bool     `json:"requires_sandbox"`
	RequiresHumanGate bool     `json:"requires_human_gate"`
}

type ToolInvocation struct {
	ToolRunID   string         `json:"tool_run_id"`
	AgentRole   string         `json:"agent_role"`
	ToolName    string         `json:"tool_name"`
	Status      string         `json:"status"`
	InputHash   string         `json:"input_hash"`
	OutputHash  string         `json:"output_hash"`
	Input       map[string]any `json:"input"`
	Output      map[string]any `json:"output"`
	EvidenceIDs []string       `json:"evidence_ids"`
	Limitations []string       `json:"limitations"`
	StartedAt   time.Time      `json:"started_at"`
	FinishedAt  time.Time      `json:"finished_at"`
}

type AgentRun struct {
	Role             string   `json:"role"`
	Status           string   `json:"status"`
	Objective        string   `json:"objective"`
	ToolRunIDs       []string `json:"tool_run_ids"`
	EvidenceIDs      []string `json:"evidence_ids"`
	Limitations      []string `json:"limitations"`
	VerdictAuthority bool     `json:"verdict_authority"`
}

type RuntimeReport struct {
	OK                   bool             `json:"ok"`
	SchemaVersion        string           `json:"schema_version"`
	Enabled              bool             `json:"enabled"`
	ShadowMode           bool             `json:"shadow_mode"`
	ExecutionMode        string           `json:"execution_mode"`
	Status               string           `json:"status"`
	CaseRef              string           `json:"case_ref"`
	Target               string           `json:"target"`
	Network              string           `json:"network"`
	GeneratedAt          time.Time        `json:"generated_at"`
	VerdictAuthority     bool             `json:"verdict_authority"`
	CanExecuteMainnet    bool             `json:"can_execute_mainnet"`
	CanModifySource      bool             `json:"can_modify_source"`
	InputHash            string           `json:"input_hash"`
	ReportHash           string           `json:"report_hash"`
	PersistenceStatus    string           `json:"persistence_status"`
	Agents               []AgentRun       `json:"agents"`
	ToolInvocations      []ToolInvocation `json:"tool_invocations"`
	AvailableTools       []ToolDefinition `json:"available_tools"`
	EvidenceIDs          []string         `json:"evidence_ids"`
	Limitations          []string         `json:"limitations"`
	RecommendedNextSteps []string         `json:"recommended_next_steps"`
}
