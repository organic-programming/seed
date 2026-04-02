package engine

// Program is parsed from program.yaml.
type Program struct {
	Description string        `yaml:"description" json:"description"`
	Steps       []ProgramStep `yaml:"steps" json:"steps"`
	MaxRetries  int           `yaml:"max_retries" json:"max_retries"` // default: 3
}

type ProgramStep struct {
	ID    string `yaml:"id" json:"id"`
	Brief string `yaml:"brief" json:"brief"` // relative path to .md file
	Gate  Gate   `yaml:"gate" json:"gate"`
}

type Gate struct {
	Command string `yaml:"command" json:"command"` // shell command (typically ader test ...)
	Expect  string `yaml:"expect" json:"expect"`   // "PASS" or "FAIL"
}

// Status is written to status.yaml.
type Status struct {
	State       string                `yaml:"state" json:"state"` // queued | running | done | deferred
	ProgramDesc string                `yaml:"program_desc" json:"program_desc"`
	CurrentStep string                `yaml:"current_step" json:"current_step"`
	Branch      string                `yaml:"branch" json:"branch"`
	StartedAt   string                `yaml:"started_at" json:"started_at"`
	FinishedAt  string                `yaml:"finished_at" json:"finished_at"`
	Steps       map[string]StepStatus `yaml:"steps" json:"steps"`
}

type StepStatus struct {
	State    string    `yaml:"state" json:"state"` // pending | running | passed | failed | skipped
	Attempts []Attempt `yaml:"attempts" json:"attempts"`
}

type Attempt struct {
	StartedAt     string `yaml:"started_at" json:"started_at"`
	FinishedAt    string `yaml:"finished_at" json:"finished_at"`
	CodexExitCode int    `yaml:"codex_exit_code" json:"codex_exit_code"`
	GateResult    string `yaml:"gate_result" json:"gate_result"` // PASS | FAIL | "" (in progress)
	GateReport    string `yaml:"gate_report" json:"gate_report"` // path to ader report
	DiffPatch     string `yaml:"diff_patch" json:"diff_patch"`   // path to saved .patch file
}

// RunOptions configures the overnight queue runner.
type RunOptions struct {
	AderRoot   string
	DryRun     bool
	MaxRetries int
}

type EnqueueOptions struct {
	AderRoot     string
	ProgramDir   string // path to a directory containing program.yaml + briefs/
	FromCookbook string // cookbook template name (mutually exclusive with ProgramDir)
}

type StatusResult struct {
	QueueSlots    []SlotSummary `json:"queue"`
	LiveSlot      *SlotSummary  `json:"live,omitempty"`
	DeferredSlots []SlotSummary `json:"deferred"`
	DoneSlots     []SlotSummary `json:"done"`
}

type SlotSummary struct {
	Slot        string `json:"slot"`
	Description string `json:"description"`
	State       string `json:"state"`
	Branch      string `json:"branch,omitempty"`
	StepsPassed int    `json:"steps_passed"`
	StepsTotal  int    `json:"steps_total"`
	CurrentStep string `json:"current_step,omitempty"`
}

type CompletionItem struct {
	Value       string
	Description string
}

type LogResult struct {
	Slot     string    `json:"slot"`
	StepID   string    `json:"step_id"`
	Attempts []Attempt `json:"attempts"`
}
