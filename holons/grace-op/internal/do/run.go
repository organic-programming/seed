package do

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/template"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/grace-op/internal/holons"
)

type Options struct {
	Params            map[string]string
	DryRun            bool
	ContinueOnError   bool
	Progress          io.Writer
	Stdout            io.Writer
	Stderr            io.Writer
	Env               []string
	ResolveRoot       *string
	ResolveSpecifiers int
	ResolveTimeout    int
}

type Result struct {
	Holon           string            `json:"holon"`
	Sequence        string            `json:"sequence"`
	Description     string            `json:"description,omitempty"`
	Dir             string            `json:"dir,omitempty"`
	DryRun          bool              `json:"dry_run,omitempty"`
	ContinueOnError bool              `json:"continue_on_error,omitempty"`
	Params          map[string]string `json:"params,omitempty"`
	Steps           []StepResult      `json:"steps,omitempty"`
}

type StepResult struct {
	Index   int    `json:"index"`
	Command string `json:"command"`
	Output  string `json:"output,omitempty"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func Run(ref, sequenceName string, opts Options) (*Result, error) {
	resolveSpecifiers := opts.ResolveSpecifiers
	if resolveSpecifiers == 0 {
		resolveSpecifiers = sdkdiscover.ALL
	}

	target, err := holons.ResolveTargetWithOptions(ref, opts.ResolveRoot, resolveSpecifiers, opts.ResolveTimeout)
	if err != nil {
		return nil, err
	}
	if target.ManifestErr != nil {
		return nil, target.ManifestErr
	}
	if target.Manifest == nil {
		return nil, fmt.Errorf("holon %q has no manifest", strings.TrimSpace(ref))
	}

	sequence, err := findSequence(target.Manifest, sequenceName)
	if err != nil {
		return nil, err
	}

	params, err := resolveParams(sequence, opts.Params)
	if err != nil {
		return nil, err
	}

	renderedSteps, err := renderSteps(sequence, params)
	if err != nil {
		return nil, err
	}

	slug := ""
	if target.Identity != nil {
		slug = target.Identity.Slug()
	}

	result := &Result{
		Holon:           slug,
		Sequence:        sequence.Name,
		Description:     strings.TrimSpace(sequence.Description),
		Dir:             target.Dir,
		DryRun:          opts.DryRun,
		ContinueOnError: opts.ContinueOnError,
		Params:          params,
		Steps:           make([]StepResult, 0, len(renderedSteps)),
	}
	if result.Holon == "" {
		result.Holon = filepathBase(target.Dir)
	}

	if opts.DryRun {
		for i, step := range renderedSteps {
			printProgress(opts.Progress, "[%d/%d] %s", i+1, len(renderedSteps), step)
			result.Steps = append(result.Steps, StepResult{
				Index:   i + 1,
				Command: step,
				Success: true,
			})
		}
		return result, nil
	}

	var failures []string
	for i, step := range renderedSteps {
		printProgress(opts.Progress, "[%d/%d] %s", i+1, len(renderedSteps), step)
		stepResult, stepErr := runStep(target.Dir, step, i+1, opts)
		result.Steps = append(result.Steps, stepResult)
		if stepErr != nil {
			printProgress(opts.Progress, "  ❌ %v", stepErr)
			failures = append(failures, fmt.Sprintf("step %d: %v", i+1, stepErr))
			if !opts.ContinueOnError {
				return result, fmt.Errorf("sequence %q failed at step %d", sequence.Name, i+1)
			}
			continue
		}
		printProgress(opts.Progress, "  ✅")
	}

	if len(failures) > 0 {
		return result, fmt.Errorf("sequence %q finished with %d failed step(s)", sequence.Name, len(failures))
	}
	return result, nil
}

func findSequence(manifest *holons.LoadedManifest, name string) (*holons.Sequence, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest required")
	}
	trimmed := strings.TrimSpace(name)
	for i := range manifest.Manifest.Sequences {
		sequence := &manifest.Manifest.Sequences[i]
		if strings.TrimSpace(sequence.Name) == trimmed {
			return sequence, nil
		}
	}
	if len(manifest.Manifest.Sequences) == 0 {
		return nil, fmt.Errorf("holon %q declares no sequences", manifest.Name)
	}
	return nil, fmt.Errorf("sequence %q not found in %s", trimmed, manifest.Name)
}

func resolveParams(sequence *holons.Sequence, provided map[string]string) (map[string]string, error) {
	params := make(map[string]string, len(sequence.Params))
	seen := make(map[string]struct{}, len(sequence.Params))

	for _, param := range sequence.Params {
		name := strings.TrimSpace(param.Name)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
		value, ok := lookupParam(provided, name)
		if !ok {
			value = strings.TrimSpace(param.Default)
			ok = value != ""
		}
		if !ok && param.Required {
			return nil, fmt.Errorf("missing required param %q", name)
		}
		params[name] = value
	}

	for name := range provided {
		if _, ok := seen[name]; !ok {
			return nil, fmt.Errorf("unknown param %q", name)
		}
	}

	return params, nil
}

func lookupParam(params map[string]string, name string) (string, bool) {
	if len(params) == 0 {
		return "", false
	}
	value, ok := params[name]
	if !ok {
		return "", false
	}
	return value, true
}

func renderSteps(sequence *holons.Sequence, params map[string]string) ([]string, error) {
	rendered := make([]string, 0, len(sequence.Steps))
	for i, step := range sequence.Steps {
		tpl, err := template.New(sequence.Name).Option("missingkey=error").Parse(step)
		if err != nil {
			return nil, fmt.Errorf("parse template for step %d: %w", i+1, err)
		}
		var buf bytes.Buffer
		if err := tpl.Execute(&buf, params); err != nil {
			return nil, fmt.Errorf("render step %d: %w", i+1, err)
		}
		rendered = append(rendered, strings.TrimSpace(buf.String()))
	}
	return rendered, nil
}

func runStep(dir, step string, index int, opts Options) (StepResult, error) {
	cmd := shellCommand(step)
	cmd.Dir = dir
	cmd.Env = commandEnv(opts.Env)

	var output bytes.Buffer
	cmd.Stdout = multiWriter(&output, opts.Stdout)
	cmd.Stderr = multiWriter(&output, opts.Stderr)

	result := StepResult{
		Index:   index,
		Command: step,
	}

	err := cmd.Run()
	result.Output = strings.TrimSpace(output.String())
	result.Success = err == nil
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	return result, nil
}

func commandEnv(values []string) []string {
	if len(values) > 0 {
		return append([]string(nil), values...)
	}
	return os.Environ()
}

func shellCommand(step string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", step)
	}
	return exec.Command("sh", "-c", step)
}

func multiWriter(buffer *bytes.Buffer, target io.Writer) io.Writer {
	if target == nil {
		return buffer
	}
	return io.MultiWriter(buffer, target)
}

func printProgress(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}

func filepathBase(path string) string {
	trimmed := strings.TrimRight(path, string(os.PathSeparator))
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, string(os.PathSeparator))
	return parts[len(parts)-1]
}
