package tools

import (
	"sort"
	"strings"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	inspectpkg "github.com/organic-programming/grace-op/internal/inspect"
)

// DefinitionsFromDescribe builds tool definitions from a HolonMeta.Describe
// response. Names are namespaced by slug to avoid collisions.
func DefinitionsFromDescribe(slug string, response *holonsv1.DescribeResponse) []Definition {
	if response == nil {
		return nil
	}

	document := inspectpkg.FromDescribeResponse(response)
	out := make([]Definition, 0)

	for _, service := range document.Services {
		serviceName := inspectpkg.ShortName(service.Name)
		if serviceName == "" {
			continue
		}
		for _, method := range service.Methods {
			if strings.TrimSpace(method.Name) == "" {
				continue
			}
			out = append(out, Definition{
				Name:        strings.TrimSpace(slug) + "." + serviceName + "." + method.Name,
				Description: strings.TrimSpace(method.Description),
				InputSchema: JSONSchemaForMethod(method),
			})
		}
	}

	for _, sequence := range document.Sequences {
		if strings.TrimSpace(sequence.Name) == "" {
			continue
		}
		out = append(out, Definition{
			Name:        strings.TrimSpace(slug) + ".sequence." + sequence.Name,
			Description: strings.TrimSpace(sequence.Description),
			InputSchema: JSONSchemaForSequence(sequence),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
