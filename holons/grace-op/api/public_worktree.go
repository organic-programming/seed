package api

import (
	"fmt"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	wt "github.com/organic-programming/grace-op/internal/worktree"
)

func Worktree(req *opv1.WorktreeRequest) (*opv1.WorktreeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("worktree request is required")
	}
	switch strings.ToLower(strings.TrimSpace(req.GetCommand())) {
	case string(wt.CommandCreate):
		mode, err := worktreeModeFromProto(req.GetMode())
		if err != nil {
			return nil, err
		}
		result, err := wt.Create(req.GetBranch(), mode)
		if err != nil {
			return nil, err
		}
		return worktreeResponseFromResult(result), nil
	case string(wt.CommandBootstrap):
		result, err := wt.Bootstrap(req.GetBranch())
		if err != nil {
			return nil, err
		}
		return worktreeResponseFromResult(result), nil
	case string(wt.CommandDoctor):
		_, result := wt.Doctor()
		return worktreeResponseFromResult(result), nil
	default:
		return nil, fmt.Errorf("worktree command must be create, bootstrap, or doctor")
	}
}

func worktreeModeFromProto(mode opv1.WorktreeMode) (wt.Mode, error) {
	switch mode {
	case opv1.WorktreeMode_WORKTREE_MODE_ISOLATED:
		return wt.ModeIsolated, nil
	case opv1.WorktreeMode_WORKTREE_MODE_PLAIN:
		return wt.ModePlain, nil
	default:
		return "", fmt.Errorf("worktree create requires mode isolated or plain")
	}
}

func worktreeResponseFromResult(result *wt.Result) *opv1.WorktreeResponse {
	if result == nil {
		return nil
	}
	resp := &opv1.WorktreeResponse{
		SchemaVersion:           int32(result.SchemaVersion),
		Command:                 result.Command,
		Mode:                    result.Mode,
		Status:                  result.Status,
		WorktreeStatus:          result.WorktreeStatus,
		Branch:                  result.Branch,
		Head:                    result.Head,
		Worktree:                result.Worktree,
		Oppath:                  result.Oppath,
		Opbin:                   result.Opbin,
		OpSha256:                result.OpSHA256,
		AderSha256:              result.AderSHA256,
		OpPath:                  result.OpPath,
		AderPath:                result.AderPath,
		ConfigChanges:           append([]string(nil), result.ConfigChanges...),
		ConfigPaths:             cloneStringMap(result.ConfigPaths),
		BootstrapJson:           result.BootstrapJSON,
		CodexConfigToml:         result.CodexConfigTOML,
		ClaudeSettingsLocalJson: result.ClaudeSettingsLocalJSON,
		GeminiEnv:               result.GeminiEnv,
		VscodeSettingsJson:      result.VSCodeSettingsJSON,
		Isolated:                result.Isolated,
		BuiltAt:                 result.BuiltAt,
	}
	if result.Activation.Cwd != "" || len(result.Activation.Env) > 0 || result.Activation.PathPrepend != "" {
		resp.Activation = &opv1.WorktreeActivation{
			Cwd:         result.Activation.Cwd,
			Env:         cloneStringMap(result.Activation.Env),
			PathPrepend: result.Activation.PathPrepend,
		}
	}
	if result.Doctor != nil {
		resp.Doctor = &opv1.WorktreeDoctor{
			Ok:              result.Doctor.OK,
			Cwd:             result.Doctor.Cwd,
			RepoRoot:        result.Doctor.RepoRoot,
			ExpectedOppath:  result.Doctor.ExpectedOPPATH,
			ExpectedOpbin:   result.Doctor.ExpectedOPBIN,
			Oppath:          result.Doctor.OPPATH,
			Opbin:           result.Doctor.OPBIN,
			OpPath:          result.Doctor.OpPath,
			AderPath:        result.Doctor.AderPath,
			Checks:          cloneBoolMap(result.Doctor.Checks),
			Error:           result.Doctor.Error,
			RecommendedCode: int32(result.Doctor.RecommendedCode),
		}
	}
	return resp
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
