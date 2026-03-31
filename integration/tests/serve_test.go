package integration

import (
	"strings"
	"testing"
)

func TestServe_OPService(t *testing.T) {
	skipIfShort(t, shortTestReason)

	sb := newSandbox(t)
	build := buildReportFor(t, sb, "gabriel-greeting-go")
	process := sb.startProcess(t, runOptions{}, "serve", "--listen", "tcp://127.0.0.1:0", "--reflect")
	defer process.Stop(t)

	address := process.waitForListenAddress(t, processStartTimeout)
	hostPort := strings.TrimPrefix(address, "tcp://")

	listResult := sb.runOP(t, "grpc://"+hostPort)
	requireSuccess(t, listResult)
	requireContains(t, listResult.Stdout, "op.v1.OPService/Discover")

	envResult := sb.runOP(t, "--format", "json", "grpc://"+hostPort, "Env", `{"shell":true}`)
	requireSuccess(t, envResult)
	envPayload := decodeJSON[map[string]any](t, envResult.Stdout)
	if envPayload["shell"] == "" {
		t.Fatalf("empty shell payload: %#v", envPayload)
	}

	discoverResult := sb.runOP(t, "--format", "json", "grpc://"+hostPort, "Discover", `{"root_dir":"`+seedRoot+`"}`)
	requireSuccess(t, discoverResult)
	discoverPayload := decodeJSON[map[string]any](t, discoverResult.Stdout)
	entries, ok := discoverPayload["entries"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("discover entries = %#v", discoverPayload["entries"])
	}

	invokeResult := sb.runOP(t, "--format", "json", "grpc://"+hostPort, "Invoke", `{"holon":"`+reportPath(build.Binary)+`","args":["SayHello","{\"name\":\"World\",\"lang_code\":\"en\"}"]}`)
	requireSuccess(t, invokeResult)
	requireContains(t, invokeResult.Stdout, "Hello")
}
