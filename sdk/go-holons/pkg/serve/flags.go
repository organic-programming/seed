package serve

import (
	"strings"

	"github.com/organic-programming/go-holons/pkg/composite"
)

type ChildSpec = composite.ChildSpec

// ParseChildFlags scans args for occurrences of "--child <slug>=<binary>" or
// "--child=<slug>=<binary>" and returns parsed children plus args with those
// flags removed.
func ParseChildFlags(args []string) (children []ChildSpec, remaining []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--child" && i+1 < len(args):
			if child, ok := parseChildSpec(args[i+1]); ok {
				children = append(children, child)
			}
			i++
		case strings.HasPrefix(arg, "--child="):
			if child, ok := parseChildSpec(strings.TrimPrefix(arg, "--child=")); ok {
				children = append(children, child)
			}
		default:
			remaining = append(remaining, arg)
		}
	}
	return children, remaining
}

func parseChildSpec(raw string) (ChildSpec, bool) {
	slug, binary, ok := strings.Cut(raw, "=")
	slug = strings.TrimSpace(slug)
	binary = strings.TrimSpace(binary)
	if !ok || slug == "" || binary == "" {
		return ChildSpec{}, false
	}
	return ChildSpec{Slug: slug, Binary: binary}, true
}
