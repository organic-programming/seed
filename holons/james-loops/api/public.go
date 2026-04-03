package api

import (
	"context"
	"fmt"
	"strings"

	jamesloopsv1 "github.com/organic-programming/james-loops/gen/go/v1"
	"github.com/organic-programming/james-loops/internal/engine"
)

func Run(req *jamesloopsv1.RunRequest) (*jamesloopsv1.RunResponse, error) {
	return runContext(context.Background(), req)
}

func Enqueue(req *jamesloopsv1.EnqueueRequest) (*jamesloopsv1.EnqueueResponse, error) {
	return enqueueContext(context.Background(), req)
}

func List(req *jamesloopsv1.ListRequest) (*jamesloopsv1.ListResponse, error) {
	return listContext(context.Background(), req)
}

func Status(req *jamesloopsv1.StatusRequest) (*jamesloopsv1.StatusResponse, error) {
	return statusContext(context.Background(), req)
}

func Drop(req *jamesloopsv1.DropRequest) (*jamesloopsv1.DropResponse, error) {
	return dropContext(context.Background(), req)
}

func Resume(req *jamesloopsv1.ResumeRequest) (*jamesloopsv1.ResumeResponse, error) {
	return resumeContext(context.Background(), req)
}

func Skip(req *jamesloopsv1.SkipRequest) (*jamesloopsv1.SkipResponse, error) {
	return skipContext(context.Background(), req)
}

func Abort(req *jamesloopsv1.AbortRequest) (*jamesloopsv1.AbortResponse, error) {
	return abortContext(context.Background(), req)
}

func ReEnqueue(req *jamesloopsv1.ReEnqueueRequest) (*jamesloopsv1.ReEnqueueResponse, error) {
	return reEnqueueContext(context.Background(), req)
}

func Log(req *jamesloopsv1.LogRequest) (*jamesloopsv1.LogResponse, error) {
	return logContext(context.Background(), req)
}

func runContext(ctx context.Context, req *jamesloopsv1.RunRequest) (*jamesloopsv1.RunResponse, error) {
	if req.GetDryRun() {
		plan, err := engine.DryRunPlan(ctx, req.GetRoot())
		if err != nil {
			return nil, err
		}
		return &jamesloopsv1.RunResponse{ReportMarkdown: renderDryRunPlan(plan)}, nil
	}
	if err := engine.Run(ctx, engine.RunOptions{
		AderRoot:         req.GetRoot(),
		DryRun:           req.GetDryRun(),
		MaxRetries:       int(req.GetMaxRetries()),
		CoderProfile:     req.GetCoderProfile(),
		EvaluatorProfile: req.GetEvaluatorProfile(),
	}); err != nil {
		return nil, err
	}
	path, markdown, err := engine.ReadMorningReport(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.RunResponse{ReportPath: path, ReportMarkdown: markdown}, nil
}

func enqueueContext(ctx context.Context, req *jamesloopsv1.EnqueueRequest) (*jamesloopsv1.EnqueueResponse, error) {
	summary, err := engine.Enqueue(ctx, engine.EnqueueOptions{
		AderRoot:     req.GetRoot(),
		ProgramDir:   req.GetProgramDir(),
		FromCookbook: req.GetFromCookbook(),
	})
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.EnqueueResponse{
		Slot:    summary.Slot,
		Summary: slotSummaryToProto(summary),
	}, nil
}

func listContext(ctx context.Context, req *jamesloopsv1.ListRequest) (*jamesloopsv1.ListResponse, error) {
	result, err := engine.List(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.ListResponse{Status: statusResultToProto(result)}, nil
}

func statusContext(ctx context.Context, req *jamesloopsv1.StatusRequest) (*jamesloopsv1.StatusResponse, error) {
	path, markdown, err := engine.ReadMorningReport(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	result, err := engine.List(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.StatusResponse{
		ReportPath:     path,
		ReportMarkdown: markdown,
		Status:         statusResultToProto(result),
	}, nil
}

func dropContext(ctx context.Context, req *jamesloopsv1.DropRequest) (*jamesloopsv1.DropResponse, error) {
	path, err := engine.Drop(ctx, req.GetRoot(), req.GetSlot(), req.GetDeferred())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.DropResponse{Slot: req.GetSlot(), Path: path}, nil
}

func resumeContext(ctx context.Context, req *jamesloopsv1.ResumeRequest) (*jamesloopsv1.ResumeResponse, error) {
	if err := engine.Resume(ctx, req.GetRoot(), 0); err != nil {
		return nil, err
	}
	path, markdown, err := engine.ReadMorningReport(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.ResumeResponse{ReportPath: path, ReportMarkdown: markdown}, nil
}

func skipContext(ctx context.Context, req *jamesloopsv1.SkipRequest) (*jamesloopsv1.SkipResponse, error) {
	skippedStep, nextStep, err := engine.Skip(ctx, req.GetRoot(), 0)
	if err != nil {
		return nil, err
	}
	path, markdown, err := engine.ReadMorningReport(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.SkipResponse{
		SkippedStep:    skippedStep,
		NextStep:       nextStep,
		ReportPath:     path,
		ReportMarkdown: markdown,
	}, nil
}

func abortContext(ctx context.Context, req *jamesloopsv1.AbortRequest) (*jamesloopsv1.AbortResponse, error) {
	slot, err := engine.Abort(ctx, req.GetRoot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.AbortResponse{DeferredSlot: slot}, nil
}

func reEnqueueContext(ctx context.Context, req *jamesloopsv1.ReEnqueueRequest) (*jamesloopsv1.ReEnqueueResponse, error) {
	summary, path, err := engine.ReEnqueue(ctx, req.GetRoot(), req.GetSlot())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.ReEnqueueResponse{
		FromSlot: req.GetSlot(),
		ToSlot:   summary.Slot,
		Path:     path,
		Summary:  slotSummaryToProto(summary),
	}, nil
}

func logContext(ctx context.Context, req *jamesloopsv1.LogRequest) (*jamesloopsv1.LogResponse, error) {
	result, err := engine.Log(ctx, req.GetRoot(), req.GetStepId())
	if err != nil {
		return nil, err
	}
	return &jamesloopsv1.LogResponse{
		Slot:     result.Slot,
		StepId:   result.StepID,
		Attempts: attemptsToProto(result.Attempts),
	}, nil
}

func renderDryRunPlan(plan []engine.SlotSummary) string {
	if len(plan) == 0 {
		return "queue is empty\n"
	}
	lines := make([]string, 0, len(plan))
	for _, item := range plan {
		lines = append(lines, fmt.Sprintf("would process %s %s", item.Slot, item.Description))
	}
	return strings.Join(lines, "\n") + "\n"
}

func statusResultToProto(result *engine.StatusResult) *jamesloopsv1.StatusSnapshot {
	if result == nil {
		return nil
	}
	queue := make([]*jamesloopsv1.SlotSummary, 0, len(result.QueueSlots))
	for _, item := range result.QueueSlots {
		queue = append(queue, slotSummaryToProto(item))
	}
	deferred := make([]*jamesloopsv1.SlotSummary, 0, len(result.DeferredSlots))
	for _, item := range result.DeferredSlots {
		deferred = append(deferred, slotSummaryToProto(item))
	}
	done := make([]*jamesloopsv1.SlotSummary, 0, len(result.DoneSlots))
	for _, item := range result.DoneSlots {
		done = append(done, slotSummaryToProto(item))
	}
	return &jamesloopsv1.StatusSnapshot{
		QueueSlots:    queue,
		LiveSlot:      slotSummaryPtrToProto(result.LiveSlot),
		DeferredSlots: deferred,
		DoneSlots:     done,
	}
}

func slotSummaryPtrToProto(summary *engine.SlotSummary) *jamesloopsv1.SlotSummary {
	if summary == nil {
		return nil
	}
	return slotSummaryToProto(*summary)
}

func slotSummaryToProto(summary engine.SlotSummary) *jamesloopsv1.SlotSummary {
	return &jamesloopsv1.SlotSummary{
		Slot:             summary.Slot,
		Description:      summary.Description,
		State:            summary.State,
		Branch:           summary.Branch,
		StepsPassed:      uint32(summary.StepsPassed),
		StepsTotal:       uint32(summary.StepsTotal),
		CurrentStep:      summary.CurrentStep,
		CoderProfile:     summary.CoderProfile,
		EvaluatorProfile: summary.EvaluatorProfile,
	}
}

func attemptsToProto(attempts []engine.Attempt) []*jamesloopsv1.AttemptRecord {
	items := make([]*jamesloopsv1.AttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		items = append(items, &jamesloopsv1.AttemptRecord{
			StartedAt:       attempt.StartedAt,
			FinishedAt:      attempt.FinishedAt,
			RunnerExitCode:  int32(attempt.CodexExitCode),
			GateResult:      attempt.GateResult,
			GateReport:      attempt.GateReport,
			DiffPatch:       attempt.DiffPatch,
			EvaluatorScore:  attempt.EvaluatorScore,
			EvaluatorOutput: attempt.EvaluatorOutput,
		})
	}
	return items
}
