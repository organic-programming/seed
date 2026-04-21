package cli

import (
	"strings"
	"testing"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"
)

func TestFormatResponse_ListIdentitiesText(t *testing.T) {
	resp := &opv1.ListIdentitiesResponse{
		Entries: []*opv1.HolonEntry{
			{
				Identity: &opv1.HolonIdentity{
					Uuid:       "12345678-90ab-cdef-1234-567890abcdef",
					GivenName:  "Alpha",
					FamilyName: "Holon",
					Clade:      opv1.Clade_DETERMINISTIC_PURE,
					Status:     opv1.Status_DRAFT,
					Lang:       "go",
				},
				Origin:       "local",
				RelativePath: "holons/alpha",
			},
		},
	}

	out := FormatResponse(FormatText, resp)
	if !strings.Contains(out, "UUID") {
		t.Fatalf("expected UUID header, got: %q", out)
	}
	if !strings.Contains(out, "Alpha Holon") {
		t.Fatalf("expected identity name, got: %q", out)
	}
	if !strings.Contains(out, "holons/alpha") {
		t.Fatalf("expected relative path, got: %q", out)
	}
}

func TestFormatResponse_DiscoverText(t *testing.T) {
	resp := &opv1.DiscoverResponse{
		Entries: []*opv1.HolonEntry{
			{
				Identity: &opv1.HolonIdentity{
					Uuid:       "87654321-90ab-cdef-1234-567890abcdef",
					GivenName:  "Who",
					FamilyName: "Holon",
					Clade:      opv1.Clade_DETERMINISTIC_PURE,
					Status:     opv1.Status_STABLE,
					Lang:       "go",
				},
				Origin:       "local",
				RelativePath: "holons/who",
			},
		},
		PathBinaries: []string{"who -> /usr/local/bin/who"},
	}

	out := FormatResponse(FormatText, resp)
	if !strings.Contains(out, "PATH binaries") {
		t.Fatalf("expected PATH section, got: %q", out)
	}
	if !strings.Contains(out, "Who Holon") {
		t.Fatalf("expected discover identity row, got: %q", out)
	}
	if !strings.Contains(out, "holons/who") {
		t.Fatalf("expected discover relative path, got: %q", out)
	}
}

func TestFormatResponse_JSON(t *testing.T) {
	resp := &opv1.CreateIdentityResponse{
		Identity: &opv1.HolonIdentity{GivenName: "Alpha"},
	}

	out := FormatResponse(FormatJSON, resp)
	if !strings.Contains(out, "givenName") {
		t.Fatalf("expected JSON output with givenName, got: %q", out)
	}
}

func TestFormatResponse_TextUnknownFallsBackToJSON(t *testing.T) {
	resp := &opv1.ShowIdentityResponse{
		Identity: &opv1.HolonIdentity{Uuid: "abc123"},
	}

	out := FormatResponse(FormatText, resp)
	if !strings.Contains(out, "abc123") {
		t.Fatalf("expected identity UUID in output, got: %q", out)
	}
}

func TestFormatRPCOutput_MethodAwareText(t *testing.T) {
	payload := []byte(`{"entries":[{"identity":{"uuid":"abc12345-0000-0000-0000-000000000000","givenName":"Alpha","familyName":"Holon","clade":"DETERMINISTIC_PURE","status":"DRAFT","lang":"go"},"origin":"local","relativePath":"holons/alpha"}]}`)
	out := formatRPCOutput(FormatText, "ListIdentities", payload)

	if !strings.Contains(out, "Alpha Holon") {
		t.Fatalf("expected text formatting, got: %q", out)
	}
}
