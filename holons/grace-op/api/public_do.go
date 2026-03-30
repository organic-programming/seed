package api

import (
	"fmt"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	dopkg "github.com/organic-programming/grace-op/internal/do"
)

func RunSequence(req *opv1.RunSequenceRequest) (*opv1.RunSequenceResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("holon and sequence are required")
	}
	if strings.TrimSpace(req.GetHolon()) == "" || strings.TrimSpace(req.GetSequence()) == "" {
		return nil, fmt.Errorf("holon and sequence are required")
	}

	result, err := dopkg.Run(req.GetHolon(), req.GetSequence(), dopkg.Options{
		Params:          req.GetParams(),
		DryRun:          req.GetDryRun(),
		ContinueOnError: req.GetContinueOnError(),
	})
	if err != nil {
		return &opv1.RunSequenceResponse{Result: doResultToProto(result)}, err
	}
	return &opv1.RunSequenceResponse{Result: doResultToProto(result)}, nil
}
