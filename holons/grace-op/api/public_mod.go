package api

import (
	"fmt"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	opmod "github.com/organic-programming/grace-op/internal/mod"
)

func ModInit(req *opv1.ModInitRequest) (*opv1.ModInitResponse, error) {
	holonPath := ""
	if req != nil {
		holonPath = req.GetHolonPath()
	}
	result, err := opmod.Init(".", holonPath)
	if err != nil {
		return nil, err
	}
	return &opv1.ModInitResponse{
		ModFile:   result.ModFile,
		HolonPath: result.HolonPath,
	}, nil
}

func ModAdd(req *opv1.ModAddRequest) (*opv1.ModAddResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("module is required")
	}
	result, err := opmod.Add(".", req.GetModule(), req.GetVersion())
	if err != nil {
		return nil, err
	}
	return &opv1.ModAddResponse{
		Dependency: dependencyToProto(result.Dependency),
		Deferred:   result.Deferred,
	}, nil
}

func ModRemove(req *opv1.ModRemoveRequest) (*opv1.ModRemoveResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("module is required")
	}
	result, err := opmod.Remove(".", req.GetModule())
	if err != nil {
		return nil, err
	}
	return &opv1.ModRemoveResponse{Path: result.Path}, nil
}

func ModTidy(_ *opv1.ModTidyRequest) (*opv1.ModTidyResponse, error) {
	result, err := opmod.Tidy(".", opmod.Options{})
	if err != nil {
		return nil, err
	}
	response := &opv1.ModTidyResponse{
		SumFile: result.SumFile,
		Pruned:  append([]string(nil), result.Pruned...),
	}
	for _, dep := range result.Current {
		response.Current = append(response.Current, dependencyToProto(dep))
	}
	return response, nil
}

func ModPull(_ *opv1.ModPullRequest) (*opv1.ModPullResponse, error) {
	result, err := opmod.Pull(".", opmod.Options{})
	if err != nil {
		return nil, err
	}
	response := &opv1.ModPullResponse{}
	for _, dep := range result.Fetched {
		response.Fetched = append(response.Fetched, dependencyToProto(dep))
	}
	return response, nil
}

func ModUpdate(req *opv1.ModUpdateRequest) (*opv1.ModUpdateResponse, error) {
	target := ""
	if req != nil {
		target = req.GetModule()
	}
	result, err := opmod.Update(".", target, opmod.Options{})
	if err != nil {
		return nil, err
	}
	response := &opv1.ModUpdateResponse{}
	for _, dep := range result.Updated {
		response.Updated = append(response.Updated, updatedDependencyToProto(dep))
	}
	return response, nil
}

func ModList(_ *opv1.ModListRequest) (*opv1.ModListResponse, error) {
	result, err := opmod.List(".")
	if err != nil {
		return nil, err
	}
	response := &opv1.ModListResponse{HolonPath: result.HolonPath}
	for _, dep := range result.Dependencies {
		response.Dependencies = append(response.Dependencies, dependencyToProto(dep))
	}
	return response, nil
}

func ModGraph(_ *opv1.ModGraphRequest) (*opv1.ModGraphResponse, error) {
	result, err := opmod.Graph(".")
	if err != nil {
		return nil, err
	}
	response := &opv1.ModGraphResponse{Root: result.Root}
	for _, edge := range result.Edges {
		response.Edges = append(response.Edges, edgeToProto(edge))
	}
	return response, nil
}
