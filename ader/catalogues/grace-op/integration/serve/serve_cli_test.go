//go:build e2e

package serve_test

import (
	"strings"
	"testing"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

func TestServe_CLI_OPService(t *testing.T) {
	integration.SkipIfShort(t, integration.ShortTestReason)

	sb := integration.NewSandbox(t)
	build := integration.BuildReportFor(t, sb, "gabriel-greeting-go")
	process := sb.StartProcess(t, integration.RunOptions{}, "serve", "--listen", "tcp://127.0.0.1:0", "--reflect")
	defer process.Stop(t)

	address := process.WaitForListenAddress(t, integration.ProcessStartTimeout)
	hostPort := strings.TrimPrefix(address, "tcp://")

	listResult := sb.RunOP(t, "grpc://"+hostPort)
	integration.RequireSuccess(t, listResult)
	integration.RequireContains(t, listResult.Stdout, "op.v1.OPService/Discover")

	envResult := sb.RunOP(t, "--format", "json", "grpc://"+hostPort, "Env", `{"shell":true}`)
	integration.RequireSuccess(t, envResult)
	envPayload := integration.DecodeJSON[map[string]any](t, envResult.Stdout)
	if envPayload["shell"] == "" {
		t.Fatalf("empty shell payload: %#v", envPayload)
	}

	discoverResult := sb.RunOP(t, "--format", "json", "grpc://"+hostPort, "Discover", `{"root_dir":"`+integration.DefaultWorkspaceDir(t)+`"}`)
	integration.RequireSuccess(t, discoverResult)
	discoverPayload := integration.DecodeJSON[map[string]any](t, discoverResult.Stdout)
	entries, ok := discoverPayload["entries"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("discover entries = %#v", discoverPayload["entries"])
	}

	invokeResult := sb.RunOP(t, "--format", "json", "grpc://"+hostPort, "Invoke", `{"holon":"`+integration.ReportPath(t, build.Binary)+`","args":["SayHello","{\"name\":\"World\",\"lang_code\":\"en\"}"]}`)
	integration.RequireSuccess(t, invokeResult)
	integration.RequireContains(t, invokeResult.Stdout, "Hello")
}
