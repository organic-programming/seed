package jamesloopsv1

type RunRequest struct {
	Root             string
	DryRun           bool
	MaxRetries       uint32
	CoderProfile     string
	EvaluatorProfile string
}

func (x *RunRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

func (x *RunRequest) GetDryRun() bool {
	if x == nil {
		return false
	}
	return x.DryRun
}

func (x *RunRequest) GetMaxRetries() uint32 {
	if x == nil {
		return 0
	}
	return x.MaxRetries
}

func (x *RunRequest) GetCoderProfile() string {
	if x == nil {
		return ""
	}
	return x.CoderProfile
}

func (x *RunRequest) GetEvaluatorProfile() string {
	if x == nil {
		return ""
	}
	return x.EvaluatorProfile
}

type RunResponse struct {
	ReportPath     string
	ReportMarkdown string
}

func (x *RunResponse) GetReportPath() string {
	if x == nil {
		return ""
	}
	return x.ReportPath
}

func (x *RunResponse) GetReportMarkdown() string {
	if x == nil {
		return ""
	}
	return x.ReportMarkdown
}

type EnqueueRequest struct {
	Root         string
	ProgramDir   string
	FromCookbook string
}

func (x *EnqueueRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

func (x *EnqueueRequest) GetProgramDir() string {
	if x == nil {
		return ""
	}
	return x.ProgramDir
}

func (x *EnqueueRequest) GetFromCookbook() string {
	if x == nil {
		return ""
	}
	return x.FromCookbook
}

type EnqueueResponse struct {
	Slot    string
	Path    string
	Summary *SlotSummary
}

func (x *EnqueueResponse) GetSlot() string {
	if x == nil {
		return ""
	}
	return x.Slot
}

func (x *EnqueueResponse) GetPath() string {
	if x == nil {
		return ""
	}
	return x.Path
}

func (x *EnqueueResponse) GetSummary() *SlotSummary {
	if x == nil {
		return nil
	}
	return x.Summary
}

type ListRequest struct {
	Root string
}

func (x *ListRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

type ListResponse struct {
	Status *StatusSnapshot
}

func (x *ListResponse) GetStatus() *StatusSnapshot {
	if x == nil {
		return nil
	}
	return x.Status
}

type StatusRequest struct {
	Root string
}

func (x *StatusRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

type StatusResponse struct {
	ReportPath     string
	ReportMarkdown string
	Status         *StatusSnapshot
}

func (x *StatusResponse) GetReportPath() string {
	if x == nil {
		return ""
	}
	return x.ReportPath
}

func (x *StatusResponse) GetReportMarkdown() string {
	if x == nil {
		return ""
	}
	return x.ReportMarkdown
}

func (x *StatusResponse) GetStatus() *StatusSnapshot {
	if x == nil {
		return nil
	}
	return x.Status
}

type DropRequest struct {
	Root     string
	Slot     string
	Deferred bool
}

func (x *DropRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

func (x *DropRequest) GetSlot() string {
	if x == nil {
		return ""
	}
	return x.Slot
}

func (x *DropRequest) GetDeferred() bool {
	if x == nil {
		return false
	}
	return x.Deferred
}

type DropResponse struct {
	Slot string
	Path string
}

func (x *DropResponse) GetSlot() string {
	if x == nil {
		return ""
	}
	return x.Slot
}

func (x *DropResponse) GetPath() string {
	if x == nil {
		return ""
	}
	return x.Path
}

type ResumeRequest struct {
	Root string
}

func (x *ResumeRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

type ResumeResponse struct {
	ReportPath     string
	ReportMarkdown string
}

func (x *ResumeResponse) GetReportPath() string {
	if x == nil {
		return ""
	}
	return x.ReportPath
}

func (x *ResumeResponse) GetReportMarkdown() string {
	if x == nil {
		return ""
	}
	return x.ReportMarkdown
}

type SkipRequest struct {
	Root string
}

func (x *SkipRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

type SkipResponse struct {
	SkippedStep    string
	NextStep       string
	ReportPath     string
	ReportMarkdown string
}

func (x *SkipResponse) GetSkippedStep() string {
	if x == nil {
		return ""
	}
	return x.SkippedStep
}

func (x *SkipResponse) GetNextStep() string {
	if x == nil {
		return ""
	}
	return x.NextStep
}

func (x *SkipResponse) GetReportPath() string {
	if x == nil {
		return ""
	}
	return x.ReportPath
}

func (x *SkipResponse) GetReportMarkdown() string {
	if x == nil {
		return ""
	}
	return x.ReportMarkdown
}

type AbortRequest struct {
	Root string
}

func (x *AbortRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

type AbortResponse struct {
	DeferredSlot string
	Path         string
}

func (x *AbortResponse) GetDeferredSlot() string {
	if x == nil {
		return ""
	}
	return x.DeferredSlot
}

func (x *AbortResponse) GetPath() string {
	if x == nil {
		return ""
	}
	return x.Path
}

type ReEnqueueRequest struct {
	Root string
	Slot string
}

func (x *ReEnqueueRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

func (x *ReEnqueueRequest) GetSlot() string {
	if x == nil {
		return ""
	}
	return x.Slot
}

type ReEnqueueResponse struct {
	FromSlot string
	ToSlot   string
	Path     string
	Summary  *SlotSummary
}

func (x *ReEnqueueResponse) GetFromSlot() string {
	if x == nil {
		return ""
	}
	return x.FromSlot
}

func (x *ReEnqueueResponse) GetToSlot() string {
	if x == nil {
		return ""
	}
	return x.ToSlot
}

func (x *ReEnqueueResponse) GetPath() string {
	if x == nil {
		return ""
	}
	return x.Path
}

func (x *ReEnqueueResponse) GetSummary() *SlotSummary {
	if x == nil {
		return nil
	}
	return x.Summary
}

type LogRequest struct {
	Root   string
	StepId string
}

func (x *LogRequest) GetRoot() string {
	if x == nil {
		return ""
	}
	return x.Root
}

func (x *LogRequest) GetStepId() string {
	if x == nil {
		return ""
	}
	return x.StepId
}

type LogResponse struct {
	Slot     string
	StepId   string
	Attempts []*AttemptRecord
}

func (x *LogResponse) GetSlot() string {
	if x == nil {
		return ""
	}
	return x.Slot
}

func (x *LogResponse) GetStepId() string {
	if x == nil {
		return ""
	}
	return x.StepId
}

func (x *LogResponse) GetAttempts() []*AttemptRecord {
	if x == nil {
		return nil
	}
	return x.Attempts
}

type StatusSnapshot struct {
	QueueSlots    []*SlotSummary
	LiveSlot      *SlotSummary
	DeferredSlots []*SlotSummary
	DoneSlots     []*SlotSummary
}

func (x *StatusSnapshot) GetQueueSlots() []*SlotSummary {
	if x == nil {
		return nil
	}
	return x.QueueSlots
}

func (x *StatusSnapshot) GetLiveSlot() *SlotSummary {
	if x == nil {
		return nil
	}
	return x.LiveSlot
}

func (x *StatusSnapshot) GetDeferredSlots() []*SlotSummary {
	if x == nil {
		return nil
	}
	return x.DeferredSlots
}

func (x *StatusSnapshot) GetDoneSlots() []*SlotSummary {
	if x == nil {
		return nil
	}
	return x.DoneSlots
}

type SlotSummary struct {
	Slot             string
	Description      string
	State            string
	Branch           string
	StepsPassed      uint32
	StepsTotal       uint32
	CurrentStep      string
	CoderProfile     string
	EvaluatorProfile string
}

func (x *SlotSummary) GetSlot() string {
	if x == nil {
		return ""
	}
	return x.Slot
}

func (x *SlotSummary) GetDescription() string {
	if x == nil {
		return ""
	}
	return x.Description
}

func (x *SlotSummary) GetState() string {
	if x == nil {
		return ""
	}
	return x.State
}

func (x *SlotSummary) GetBranch() string {
	if x == nil {
		return ""
	}
	return x.Branch
}

func (x *SlotSummary) GetStepsPassed() uint32 {
	if x == nil {
		return 0
	}
	return x.StepsPassed
}

func (x *SlotSummary) GetStepsTotal() uint32 {
	if x == nil {
		return 0
	}
	return x.StepsTotal
}

func (x *SlotSummary) GetCurrentStep() string {
	if x == nil {
		return ""
	}
	return x.CurrentStep
}

func (x *SlotSummary) GetCoderProfile() string {
	if x == nil {
		return ""
	}
	return x.CoderProfile
}

func (x *SlotSummary) GetEvaluatorProfile() string {
	if x == nil {
		return ""
	}
	return x.EvaluatorProfile
}

type AttemptRecord struct {
	StartedAt       string
	FinishedAt      string
	RunnerExitCode  int32
	GateResult      string
	GateReport      string
	DiffPatch       string
	EvaluatorScore  float64
	EvaluatorOutput string
}

func (x *AttemptRecord) GetStartedAt() string {
	if x == nil {
		return ""
	}
	return x.StartedAt
}

func (x *AttemptRecord) GetFinishedAt() string {
	if x == nil {
		return ""
	}
	return x.FinishedAt
}

func (x *AttemptRecord) GetRunnerExitCode() int32 {
	if x == nil {
		return 0
	}
	return x.RunnerExitCode
}

func (x *AttemptRecord) GetGateResult() string {
	if x == nil {
		return ""
	}
	return x.GateResult
}

func (x *AttemptRecord) GetGateReport() string {
	if x == nil {
		return ""
	}
	return x.GateReport
}

func (x *AttemptRecord) GetDiffPatch() string {
	if x == nil {
		return ""
	}
	return x.DiffPatch
}

func (x *AttemptRecord) GetEvaluatorScore() float64 {
	if x == nil {
		return 0
	}
	return x.EvaluatorScore
}

func (x *AttemptRecord) GetEvaluatorOutput() string {
	if x == nil {
		return ""
	}
	return x.EvaluatorOutput
}
