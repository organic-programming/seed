package api

import (
	"encoding/json"
	"errors"

	"github.com/organic-programming/grace-op/internal/holons"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActiveArtifactConflict identifies processes that must exit before a build or
// clean can modify their artifacts. API callers can inspect it with errors.As.
// API calls never prompt for consent or stop these processes.
type ActiveArtifactConflict = holons.ActiveArtifactConflict

// ActiveArtifactProcess is a process using an artifact in a conflicting build.
type ActiveArtifactProcess = holons.ActiveArtifactProcess

// Preserve ordinary errors and convert only native artifact conflicts. In
// particular, a recipe failure is not evidence that a process should be stopped.
func activeArtifactRPCError(err error) error {
	var conflict *holons.ActiveArtifactConflict
	if !errors.As(err, &conflict) {
		return err
	}
	processes, marshalErr := json.Marshal(conflict.Processes)
	if marshalErr != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	s, detailErr := status.New(codes.FailedPrecondition, err.Error()).WithDetails(&errdetails.ErrorInfo{
		Reason: "ACTIVE_ARTIFACTS",
		Domain: "organic-programming.org/op",
		Metadata: map[string]string{
			"holon":     conflict.Holon,
			"operation": string(conflict.Operation),
			"processes": string(processes),
		},
	})
	if detailErr != nil {
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return s.Err()
}
