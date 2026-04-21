package api

import (
	"fmt"
	"path/filepath"
	"strings"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/scaffold"
	"github.com/organic-programming/grace-op/internal/who"
)

func Discover(req *opv1.DiscoverRequest) (*opv1.DiscoverResponse, error) {
	root := "."
	if req != nil && strings.TrimSpace(req.GetRootDir()) != "" {
		root = req.GetRootDir()
	}

	local, err := holons.DiscoverHolons(root)
	if err != nil {
		return nil, err
	}
	cached, err := holons.DiscoverCachedHolons()
	if err != nil {
		return nil, err
	}

	entries := make([]*opv1.HolonEntry, 0, len(local)+len(cached))
	for _, entry := range local {
		entries = append(entries, localHolonToProto(entry))
	}
	for _, entry := range cached {
		entries = append(entries, localHolonToProto(entry))
	}

	return &opv1.DiscoverResponse{
		Entries:           entries,
		PathBinaries:      holons.DiscoverInPath(),
		InstalledBinaries: holons.DiscoverInOPBIN(),
	}, nil
}

func ListIdentities(req *opv1.ListIdentitiesRequest) (*opv1.ListIdentitiesResponse, error) {
	root := "."
	if req != nil && strings.TrimSpace(req.GetRootDir()) != "" {
		root = req.GetRootDir()
	}
	return who.List(root)
}

func ShowIdentity(req *opv1.ShowIdentityRequest) (*opv1.ShowIdentityResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("uuid is required")
	}
	return who.Show(req.GetUuid())
}

func CreateIdentity(req *opv1.CreateIdentityRequest) (*opv1.CreateIdentityResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("identity request is required")
	}
	return who.Create(req)
}

func ListTemplates(_ *opv1.ListTemplatesRequest) (*opv1.ListTemplatesResponse, error) {
	entries, err := scaffold.List()
	if err != nil {
		return nil, err
	}

	response := &opv1.ListTemplatesResponse{
		Entries: make([]*opv1.TemplateEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		response.Entries = append(response.Entries, templateEntryToProto(entry))
	}
	return response, nil
}

func GenerateTemplate(req *opv1.GenerateTemplateRequest) (*opv1.GenerateTemplateResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("template and slug are required")
	}

	result, err := scaffold.Generate(req.GetTemplate(), req.GetSlug(), scaffold.GenerateOptions{
		Dir:       req.GetDir(),
		Overrides: req.GetOverrides(),
	})
	if err != nil {
		return nil, err
	}

	dir := result.Dir
	if absDir, err := filepath.Abs(dir); err == nil {
		dir = absDir
	}
	return &opv1.GenerateTemplateResponse{
		Template: result.Template,
		Dir:      dir,
	}, nil
}
