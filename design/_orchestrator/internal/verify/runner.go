package verify

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Command  string
	Passed   bool
	Output   string
	Duration time.Duration
}

func Run(commands []string, workDir string, timeout time.Duration) []Result {
	results := make([]Result, 0, len(commands))
	for _, command := range commands {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		cancel()

		result := Result{
			Command:  command,
			Passed:   err == nil,
			Output:   strings.TrimSpace(string(output)),
			Duration: time.Since(start),
		}
		if ctx.Err() == context.DeadlineExceeded {
			result.Passed = false
			if result.Output != "" {
				result.Output += "\n"
			}
			result.Output += fmt.Sprintf("verification timed out after %s", timeout)
		} else if err != nil && result.Output == "" {
			result.Output = err.Error()
		}
		results = append(results, result)
	}

	return results
}
