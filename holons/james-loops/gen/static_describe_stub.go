//go:build jamesloops_stubs

package gen

import holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"

// StaticDescribeResponse is a local fallback used until op build materializes gen/.
func StaticDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "b3b3b9f4-110c-45e7-922e-5e60a39ef64e",
				GivenName:  "James",
				FamilyName: "Loops",
				Motto:      "Queue the briefs, run the night, read the morning report.",
				Composer:   "B. ALTER",
				Status:     "draft",
				Born:       "2026-04-03",
				Version:    "0.1.1",
				Aliases:    []string{"james-loops"},
			},
			Description: "James Loops orchestrates sequential overnight AI CLI sessions governed by ader acceptance gates.",
			Lang:        "go",
			Kind:        "native",
			Artifacts: &holonsv1.HolonManifest_Artifacts{
				Binary: "james-loops",
			},
		},
	}
}
