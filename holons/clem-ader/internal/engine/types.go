package engine

type RunOptions struct {
	ConfigDir     string
	Suite         string
	Profile       string
	Lane          string
	StepFilter    string
	Source        string
	ArchivePolicy string
	KeepReport    bool
	KeepSnapshot  bool
}

type ArchiveOptions struct {
	ConfigDir string
	HistoryID string
	Latest    bool
}

type RunResult struct {
	Manifest        HistoryRecord
	Steps           []StepResult
	SummaryMarkdown string
	SummaryTSV      string
	Promotion       *PromotionProposal
}

type HistoryRecord struct {
	ConfigDir     string `json:"config_dir"`
	Suite         string `json:"suite"`
	HistoryID     string `json:"history_id"`
	Profile       string `json:"profile"`
	Lane          string `json:"lane"`
	Source        string `json:"source"`
	ArchivePolicy string `json:"archive_policy"`
	StepFilter    string `json:"step_filter"`
	RepoRoot      string `json:"repo_root"`
	SnapshotRoot  string `json:"snapshot_root"`
	ReportDir     string `json:"report_dir"`
	ArchivePath   string `json:"archive_path"`
	CommitHash    string `json:"commit_hash"`
	Branch        string `json:"branch"`
	Dirty         bool   `json:"dirty"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
	FinalStatus   string `json:"final_status"`
	PassCount     int    `json:"pass_count"`
	FailCount     int    `json:"fail_count"`
	SkipCount     int    `json:"skip_count"`
}

type StepResult struct {
	StepID          string `json:"step_id"`
	Lane            string `json:"lane,omitempty"`
	Description     string `json:"description"`
	Workdir         string `json:"workdir"`
	Command         string `json:"command"`
	Status          string `json:"status"`
	Reason          string `json:"reason,omitempty"`
	LogPath         string `json:"log_path"`
	StartedAt       string `json:"started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
	DurationSeconds int64  `json:"duration_seconds"`
}

type HistoryEntry struct {
	HistoryID   string `json:"history_id"`
	Suite       string `json:"suite"`
	Profile     string `json:"profile"`
	Lane        string `json:"lane"`
	Source      string `json:"source"`
	FinalStatus string `json:"final_status"`
	CommitHash  string `json:"commit_hash"`
	Dirty       bool   `json:"dirty"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
	ReportDir   string `json:"report_dir"`
	ArchivePath string `json:"archive_path"`
}

type CleanupResult struct {
	RemovedLocalSuiteDirs int      `json:"removed_local_suite_dirs"`
	RemovedTempStores     int      `json:"removed_temp_stores"`
	RemovedTempAliases    int      `json:"removed_temp_aliases"`
	RemovedPaths          []string `json:"removed_paths"`
}

type StepSpec struct {
	ID          string
	Lane        string
	Workdir     string
	Prereqs     []string
	Command     string
	Script      string
	Args        []string
	Description string
}

type PromotionProposal struct {
	Suite                  string   `json:"suite"`
	Profile                string   `json:"profile"`
	Lane                   string   `json:"lane"`
	DestinationLane        string   `json:"destination_lane"`
	SuiteFile              string   `json:"suite_file"`
	EligibleSteps          []string `json:"eligible_steps"`
	SuggestedPatch         string   `json:"suggested_patch"`
	SuggestedGitCommands   []string `json:"suggested_git_commands"`
	SuggestedCommitMessage string   `json:"suggested_commit_message"`
}
