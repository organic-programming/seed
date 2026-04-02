//go:build !codexloops_generated

package gen

import holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"

// StaticDescribeResponse is a local fallback used until op build materializes gen/.
func StaticDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "22e33ab1-4737-4b79-b018-53efed7976a2",
				GivenName:  "Codex",
				FamilyName: "Loops",
				Motto:      "Queue the briefs, run the night, read the morning report.",
				Composer:   "B. ALTER",
				Status:     "draft",
				Born:       "2026-04-02",
				Version:    "0.1.0",
				Aliases:    []string{"codex-loops"},
			},
			Description: "Codex Loops orchestrates sequential overnight Codex CLI sessions governed by ader acceptance gates.",
			Lang:        "go",
			Kind:        "native",
			Artifacts: &holonsv1.HolonManifest_Artifacts{
				Binary: "codex-loops",
			},
		},
	}
}
