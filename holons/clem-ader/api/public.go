package api

import (
	"context"

	aderv1 "github.com/organic-programming/clem-ader/gen/go/v1"
	"github.com/organic-programming/clem-ader/internal/engine"
)

func Test(req *aderv1.TestRequest) (*aderv1.TestResponse, error) {
	return testContext(context.Background(), req)
}

func Archive(req *aderv1.ArchiveRequest) (*aderv1.ArchiveResponse, error) {
	return archiveContext(context.Background(), req)
}

func Cleanup(req *aderv1.CleanupRequest) (*aderv1.CleanupResponse, error) {
	return cleanupContext(context.Background(), req)
}

func ListRuns(req *aderv1.ListRunsRequest) (*aderv1.ListRunsResponse, error) {
	return listRunsContext(context.Background(), req)
}

func ShowRun(req *aderv1.ShowRunRequest) (*aderv1.ShowRunResponse, error) {
	return showRunContext(context.Background(), req)
}

func testContext(ctx context.Context, req *aderv1.TestRequest) (*aderv1.TestResponse, error) {
	result, err := engine.Run(ctx, engine.RunOptions{
		ConfigDir:     req.GetConfigDir(),
		Suite:         req.GetSuite(),
		Profile:       req.GetProfile(),
		Lane:          req.GetLane(),
		StepFilter:    req.GetStepFilter(),
		Source:        req.GetSource(),
		ArchivePolicy: req.GetArchivePolicy(),
		KeepReport:    req.GetKeepReport(),
		KeepSnapshot:  req.GetKeepSnapshot(),
	})
	if err != nil {
		return nil, err
	}
	return &aderv1.TestResponse{
		Manifest: manifestToProto(result.Manifest),
		Steps:    stepsToProto(result.Steps),
	}, nil
}

func archiveContext(ctx context.Context, req *aderv1.ArchiveRequest) (*aderv1.ArchiveResponse, error) {
	result, err := engine.Archive(ctx, engine.ArchiveOptions{
		ConfigDir: req.GetConfigDir(),
		RunID:     req.GetRunId(),
		Latest:    req.GetLatest(),
	})
	if err != nil {
		return nil, err
	}
	return &aderv1.ArchiveResponse{
		Manifest:    manifestToProto(result.Manifest),
		ArchivePath: result.Manifest.ArchivePath,
	}, nil
}

func cleanupContext(ctx context.Context, req *aderv1.CleanupRequest) (*aderv1.CleanupResponse, error) {
	result, err := engine.Cleanup(ctx, req.GetConfigDir())
	if err != nil {
		return nil, err
	}
	return &aderv1.CleanupResponse{
		RemovedLocalSuiteDirs: uint32(result.RemovedLocalSuiteDirs),
		RemovedTempStores:     uint32(result.RemovedTempStores),
		RemovedTempAliases:    uint32(result.RemovedTempAliases),
		RemovedPaths:          append([]string(nil), result.RemovedPaths...),
	}, nil
}

func listRunsContext(ctx context.Context, req *aderv1.ListRunsRequest) (*aderv1.ListRunsResponse, error) {
	runs, err := engine.ListRuns(ctx, req.GetConfigDir())
	if err != nil {
		return nil, err
	}
	return &aderv1.ListRunsResponse{Runs: runSummariesToProto(runs)}, nil
}

func showRunContext(ctx context.Context, req *aderv1.ShowRunRequest) (*aderv1.ShowRunResponse, error) {
	result, err := engine.ShowRun(ctx, req.GetConfigDir(), req.GetRunId())
	if err != nil {
		return nil, err
	}
	return &aderv1.ShowRunResponse{
		Manifest:        manifestToProto(result.Manifest),
		Steps:           stepsToProto(result.Steps),
		SummaryMarkdown: result.SummaryMarkdown,
		SummaryTsv:      result.SummaryTSV,
	}, nil
}

func manifestToProto(m engine.RunManifest) *aderv1.RunManifest {
	return &aderv1.RunManifest{
		ConfigDir:     m.ConfigDir,
		Suite:         m.Suite,
		RunId:         m.RunID,
		Profile:       m.Profile,
		Lane:          m.Lane,
		Source:        m.Source,
		ArchivePolicy: m.ArchivePolicy,
		StepFilter:    m.StepFilter,
		RepoRoot:      m.RepoRoot,
		SnapshotRoot:  m.SnapshotRoot,
		ReportDir:     m.ReportDir,
		ArchivePath:   m.ArchivePath,
		CommitHash:    m.CommitHash,
		Branch:        m.Branch,
		Dirty:         m.Dirty,
		StartedAt:     m.StartedAt,
		FinishedAt:    m.FinishedAt,
		FinalStatus:   m.FinalStatus,
		PassCount:     uint32(m.PassCount),
		FailCount:     uint32(m.FailCount),
		SkipCount:     uint32(m.SkipCount),
	}
}

func stepsToProto(steps []engine.StepResult) []*aderv1.StepResult {
	out := make([]*aderv1.StepResult, 0, len(steps))
	for _, step := range steps {
		out = append(out, &aderv1.StepResult{
			StepId:          step.StepID,
			Lane:            step.Lane,
			Description:     step.Description,
			Workdir:         step.Workdir,
			Command:         step.Command,
			Status:          step.Status,
			Reason:          step.Reason,
			LogPath:         step.LogPath,
			StartedAt:       step.StartedAt,
			FinishedAt:      step.FinishedAt,
			DurationSeconds: uint64(step.DurationSeconds),
		})
	}
	return out
}

func runSummariesToProto(items []engine.RunSummary) []*aderv1.RunSummary {
	out := make([]*aderv1.RunSummary, 0, len(items))
	for _, item := range items {
		out = append(out, &aderv1.RunSummary{
			RunId:       item.RunID,
			Suite:       item.Suite,
			Profile:     item.Profile,
			Lane:        item.Lane,
			Source:      item.Source,
			FinalStatus: item.FinalStatus,
			CommitHash:  item.CommitHash,
			Dirty:       item.Dirty,
			StartedAt:   item.StartedAt,
			FinishedAt:  item.FinishedAt,
			ReportDir:   item.ReportDir,
			ArchivePath: item.ArchivePath,
		})
	}
	return out
}
