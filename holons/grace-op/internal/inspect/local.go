package inspect

import (
	"path/filepath"
	"strings"

	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	"github.com/organic-programming/grace-op/internal/holons"
	"github.com/organic-programming/grace-op/internal/identity"
)

// LocalCatalog is a parsed local holon ready for inspect/tools/mcp reuse.
type LocalCatalog struct {
	*Catalog
	Dir  string
	Slug string
}

// LoadLocal resolves a slug/path/uuid selector, parses the holon's protos, and
// attaches identity metadata, skills, and sequences from the manifest source.
func LoadLocal(ref string) (*LocalCatalog, error) {
	return LoadLocalWithOptions(ref, nil, sdkdiscover.ALL, sdkdiscover.NO_TIMEOUT)
}

func LoadLocalWithOptions(ref string, root *string, specifiers int, timeout int) (*LocalCatalog, error) {
	target, err := holons.ResolveTargetWithOptions(ref, root, specifiers, timeout)
	if err != nil {
		return nil, err
	}

	protoDir := filepath.Join(target.Dir, "protos")
	if filepath.Base(target.IdentityPath) == identity.ProtoManifestFileName {
		protoDir = target.Dir
	}

	catalog, err := ParseCatalog(protoDir)
	if err != nil {
		return nil, err
	}

	slug := filepath.Base(target.Dir)
	catalog.Document.Slug = slug
	if target.Identity != nil && strings.TrimSpace(target.Identity.Motto) != "" {
		catalog.Document.Motto = strings.TrimSpace(target.Identity.Motto)
	}
	if target.Manifest != nil {
		catalog.Document.Skills = manifestSkills(target.Manifest.Manifest.Skills)
		catalog.Document.Sequences = manifestSequences(target.Manifest.Manifest.Sequences)
	}

	return &LocalCatalog{
		Catalog: catalog,
		Dir:     target.Dir,
		Slug:    slug,
	}, nil
}

func manifestSkills(skills []holons.Skill) []Skill {
	out := make([]Skill, 0, len(skills))
	for _, skill := range skills {
		out = append(out, Skill{
			Name:        strings.TrimSpace(skill.Name),
			Description: strings.TrimSpace(skill.Description),
			When:        strings.TrimSpace(skill.When),
			Steps:       append([]string(nil), skill.Steps...),
		})
	}
	return out
}

func manifestSequences(sequences []holons.Sequence) []Sequence {
	out := make([]Sequence, 0, len(sequences))
	for _, sequence := range sequences {
		params := make([]SequenceParam, 0, len(sequence.Params))
		for _, param := range sequence.Params {
			params = append(params, SequenceParam{
				Name:        strings.TrimSpace(param.Name),
				Description: strings.TrimSpace(param.Description),
				Required:    param.Required,
				Default:     strings.TrimSpace(param.Default),
			})
		}
		out = append(out, Sequence{
			Name:        strings.TrimSpace(sequence.Name),
			Description: strings.TrimSpace(sequence.Description),
			Params:      params,
			Steps:       append([]string(nil), sequence.Steps...),
		})
	}
	return out
}
