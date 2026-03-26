// Package identity resolves holon identity fields from holon.proto manifests.
package identity

import (
	"strings"
)

// Identity contains the identity and lineage fields from holon.proto.
type Identity struct {
	UUID         string
	GivenName    string
	FamilyName   string
	Motto        string
	Composer     string
	Clade        string
	Status       string
	Born         string
	Version      string // semver, no "v" prefix
	Lang         string
	Parents      []string
	Reproduction string
	GeneratedBy  string
	ProtoStatus  string
	Aliases      []string
}

// Slug returns the canonical holon slug derived from the identity.
func (id Identity) Slug() string {
	given := strings.TrimSpace(id.GivenName)
	family := strings.TrimSpace(strings.TrimSuffix(id.FamilyName, "?"))
	if given == "" && family == "" {
		return ""
	}

	slug := strings.TrimSpace(given + "-" + family)
	slug = strings.ToLower(slug)
	slug = strings.ReplaceAll(slug, " ", "-")
	return strings.Trim(slug, "-")
}
