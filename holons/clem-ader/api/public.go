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

func History(req *aderv1.HistoryRequest) (*aderv1.HistoryResponse, error) {
	return historyContext(context.Background(), req)
}

func ShowHistory(req *aderv1.ShowHistoryRequest) (*aderv1.ShowHistoryResponse, error) {
	return showHistoryContext(context.Background(), req)
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
		HistoryID: req.GetHistoryId(),
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

func historyContext(ctx context.Context, req *aderv1.HistoryRequest) (*aderv1.HistoryResponse, error) {
	entries, err := engine.History(ctx, req.GetConfigDir())
	if err != nil {
		return nil, err
	}
	return &aderv1.HistoryResponse{Entries: historyEntriesToProto(entries)}, nil
}

func showHistoryContext(ctx context.Context, req *aderv1.ShowHistoryRequest) (*aderv1.ShowHistoryResponse, error) {
	result, err := engine.ShowHistory(ctx, req.GetConfigDir(), req.GetHistoryId())
	if err != nil {
		return nil, err
	}
	return &aderv1.ShowHistoryResponse{
		Manifest:        manifestToProto(result.Manifest),
		Steps:           stepsToProto(result.Steps),
		SummaryMarkdown: result.SummaryMarkdown,
		SummaryTsv:      result.SummaryTSV,
	}, nil
}

func manifestToProto(m engine.HistoryRecord) *aderv1.HistoryRecord {
	return &aderv1.HistoryRecord{
		ConfigDir:     m.ConfigDir,
		Suite:         m.Suite,
		HistoryId:     m.HistoryID,
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

func historyEntriesToProto(items []engine.HistoryEntry) []*aderv1.HistoryEntry {
	out := make([]*aderv1.HistoryEntry, 0, len(items))
	for _, item := range items {
		out = append(out, &aderv1.HistoryEntry{
			HistoryId:   item.HistoryID,
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
