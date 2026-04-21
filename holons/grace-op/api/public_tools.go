package api

import (
	"fmt"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
	toolspkg "github.com/organic-programming/grace-op/internal/tools"
)

func Tools(req *opv1.ToolsRequest) (*opv1.ToolsResponse, error) {
	if req == nil || strings.TrimSpace(req.GetTarget()) == "" {
		return nil, fmt.Errorf("target is required")
	}

	format := toolspkg.FormatOpenAI
	if strings.TrimSpace(req.GetFormat()) != "" {
		parsed, err := toolspkg.ParseFormat(req.GetFormat())
		if err != nil {
			return nil, err
		}
		format = parsed
	}

	catalog, err := inspectpkg.LoadLocal(req.GetTarget())
	if err != nil {
		return nil, err
	}

	payload, err := toolspkg.MarshalDefinitions(
		toolspkg.DefinitionsForCatalogs([]*inspectpkg.LocalCatalog{catalog}),
		format,
	)
	if err != nil {
		return nil, err
	}

	return &opv1.ToolsResponse{
		Format: format,
		Payload: payload,
	}, nil
}
