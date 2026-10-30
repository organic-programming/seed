package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/organic-programming/grace-op/internal/holons"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestActiveArtifactRPCErrorProvidesStructuredDiagnostic(t *testing.T) {
	conflict := &ActiveArtifactConflict{
		Operation: holons.OperationBuild,
		Holon:     "demo",
		Processes: []ActiveArtifactProcess{{PID: 42, Path: "/tmp/Projet été/.op/build/Demo.app/Contents/MacOS/Demo", Started: "123:456", BundlePath: "/tmp/Projet été/.op/build/Demo.app"}},
	}
	err := activeArtifactRPCError(fmt.Errorf("build: %w", conflict))
	s := status.Convert(err)
	if s.Code() != codes.FailedPrecondition {
		t.Fatalf("code = %s, want FailedPrecondition", s.Code())
	}
	if s.Message() != "build: "+conflict.Error() {
		t.Fatalf("message = %q", s.Message())
	}
	if len(s.Details()) != 1 {
		t.Fatalf("details = %#v", s.Details())
	}
	detail, ok := s.Details()[0].(*errdetails.ErrorInfo)
	if !ok || detail.GetReason() != "ACTIVE_ARTIFACTS" || detail.GetDomain() != "organic-programming.org/op" {
		t.Fatalf("detail = %#v", s.Details()[0])
	}
	if detail.Metadata["holon"] != "demo" || detail.Metadata["operation"] != "build" {
		t.Fatalf("metadata = %#v", detail.Metadata)
	}
	var processes []ActiveArtifactProcess
	if err := json.Unmarshal([]byte(detail.Metadata["processes"]), &processes); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(processes, conflict.Processes) {
		t.Fatalf("processes = %+v, want %+v", processes, conflict.Processes)
	}
}

func TestActiveArtifactRPCErrorPreservesOtherFailures(t *testing.T) {
	for _, err := range []error{nil, errors.New("recipe failed"), errors.New("Bad state: Refusing to rebuild or replace files used by running processes.")} {
		if got := activeArtifactRPCError(err); got != err {
			t.Fatalf("ordinary error %v converted to %v", err, got)
		}
	}
}
