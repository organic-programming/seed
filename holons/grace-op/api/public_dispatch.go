package api

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/holons"
)

func Invoke(req *opv1.InvokeRequest) (*opv1.InvokeResponse, error) {
	return invokeContext(context.Background(), req)
}

func invokeContext(ctx context.Context, req *opv1.InvokeRequest) (*opv1.InvokeResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("holon is required")
	}

	binary, err := holons.ResolveBinary(req.GetHolon())
	if err != nil {
		return &opv1.InvokeResponse{
			ExitCode: 1,
			Stderr:   fmt.Sprintf("holon %q not found", strings.TrimSpace(req.GetHolon())),
		}, nil
	}

	cmd := exec.CommandContext(ctx, binary, req.GetArgs()...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := int32(0)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitErr.ExitCode())
		} else {
			return nil, fmt.Errorf("failed to run %s: %w", req.GetHolon(), err)
		}
	}

	return &opv1.InvokeResponse{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}
