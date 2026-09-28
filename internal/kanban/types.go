package kanban

var StatusColumns = []string{"triage", "todo", "ready", "running", "blocked", "scheduled", "review", "done"}

type Task struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Assignee           *string  `json:"assignee"`
	Status             string   `json:"status"`
	Priority           int      `json:"priority"`
	Tenant             *string  `json:"tenant"`
	WorkspaceKind      string   `json:"workspace_kind"`
	WorkspacePath      *string  `json:"workspace_path"`
	BranchName         *string  `json:"branch_name"`
	ProjectID          *string  `json:"project_id"`
	CreatedBy          string   `json:"created_by"`
	CreatedAt          int64    `json:"created_at"`
	StartedAt          *int64   `json:"started_at"`
	CompletedAt        *int64   `json:"completed_at"`
	Result             *string  `json:"result"`
	Skills             []string `json:"skills"`
	MaxRuntimeSeconds  *int     `json:"max_runtime_seconds"`
	MaxRetries         *int     `json:"max_retries"`
	ModelOverride      *string  `json:"model_override"`
	ProviderOverride   *string  `json:"provider_override"`
	SessionID          *string  `json:"session_id"`
	WorkflowTemplateID *string  `json:"workflow_template_id"`
	CurrentStepKey     *string  `json:"current_step_key"`
	CompletionContract string   `json:"completion_contract"`
	LastFailureError   *string  `json:"last_failure_error"`
}

type Board struct {
	Slug           string         `json:"slug"`
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	Icon           string         `json:"icon"`
	Color          string         `json:"color"`
	DefaultWorkdir *string        `json:"default_workdir"`
	ProjectID      *string        `json:"project_id"`
	CreatedAt      *int64         `json:"created_at"`
	Archived       bool           `json:"archived"`
	DBPath         string         `json:"db_path"`
	Current        bool           `json:"is_current"`
	Counts         map[string]int `json:"counts"`
	Total          int            `json:"total"`
}

type Comment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"`
}
type Event struct {
	Kind      string `json:"kind"`
	Payload   any    `json:"payload"`
	CreatedAt int64  `json:"created_at"`
}
type Detail struct {
	Task          Task      `json:"task"`
	LatestSummary *string   `json:"latest_summary"`
	Parents       []string  `json:"parents"`
	Children      []string  `json:"children"`
	Comments      []Comment `json:"comments"`
	Events        []Event   `json:"events"`
	Runs          []any     `json:"runs"`
}
