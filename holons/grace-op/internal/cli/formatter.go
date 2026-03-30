package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	opv1 "github.com/organic-programming/grace-op/gen/go/op/v1"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Format determines how to display a protobuf response.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// FormatResponse formats a gRPC response for CLI output.
func FormatResponse(format Format, resp proto.Message) string {
	if resp == nil {
		return ""
	}

	if format == FormatJSON {
		return marshalProtoJSONForOutput(resp)
	}

	switch typed := resp.(type) {
	case *opv1.ListIdentitiesResponse:
		return formatListIdentitiesText(typed)
	case *opv1.ShowIdentityResponse:
		return formatShowIdentityText(typed)
	case *opv1.CreateIdentityResponse:
		return formatCreateIdentityText(typed)
	case *opv1.DiscoverResponse:
		return formatDiscoverText(typed)
	default:
		return marshalProtoJSONForOutput(resp)
	}
}

func formatRPCOutput(format Format, method string, payload []byte) string {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return ""
	}

	resp := responseMessageForMethod(method)
	if resp == nil {
		return normalizeJSON(trimmed)
	}
	if err := protojson.Unmarshal([]byte(trimmed), resp); err != nil {
		return normalizeJSON(trimmed)
	}

	return FormatResponse(format, resp)
}

func responseMessageForMethod(method string) proto.Message {
	switch canonicalMethodName(method) {
	case "CreateIdentity":
		return &opv1.CreateIdentityResponse{}
	case "ListIdentities":
		return &opv1.ListIdentitiesResponse{}
	case "ShowIdentity":
		return &opv1.ShowIdentityResponse{}
	case "Discover":
		return &opv1.DiscoverResponse{}
	default:
		return nil
	}
}

func formatCreateIdentityText(resp *opv1.CreateIdentityResponse) string {
	var b strings.Builder
	b.WriteString("Identity created\n")
	if resp.GetFilePath() != "" {
		fmt.Fprintf(&b, "File: %s\n", resp.GetFilePath())
	}
	appendIdentityTable(&b, resp.GetIdentity())
	return strings.TrimSpace(b.String())
}

func formatShowIdentityText(resp *opv1.ShowIdentityResponse) string {
	var b strings.Builder
	if resp.GetFilePath() != "" {
		fmt.Fprintf(&b, "File: %s\n", resp.GetFilePath())
	}
	appendIdentityTable(&b, resp.GetIdentity())
	if resp.GetRawContent() != "" {
		fmt.Fprintf(&b, "Raw content bytes: %d", len(resp.GetRawContent()))
	}
	return strings.TrimSpace(b.String())
}

func formatListIdentitiesText(resp *opv1.ListIdentitiesResponse) string {
	if len(resp.GetEntries()) == 0 {
		return "No identities found."
	}

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tUUID\tNAME\tCLADE\tSTATUS\tLANG\tORIGIN\tPATH")
	for _, entry := range resp.GetEntries() {
		id := entry.GetIdentity()
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			defaultDash(identitySlug(id)),
			shortUUID(id.GetUuid()),
			displayName(id),
			cladeLabel(id.GetClade()),
			statusLabel(id.GetStatus()),
			defaultDash(id.GetLang()),
			defaultDash(entry.GetOrigin()),
			defaultDash(entry.GetRelativePath()),
		)
	}
	_ = w.Flush()
	return strings.TrimSpace(b.String())
}

func formatDiscoverText(resp *opv1.DiscoverResponse) string {
	var b strings.Builder

	if len(resp.GetEntries()) > 0 {
		w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tUUID\tNAME\tCLADE\tSTATUS\tLANG\tORIGIN\tPATH")
		for _, entry := range resp.GetEntries() {
			id := entry.GetIdentity()
			fmt.Fprintf(
				w,
				"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				defaultDash(identitySlug(id)),
				shortUUID(id.GetUuid()),
				displayName(id),
				cladeLabel(id.GetClade()),
				statusLabel(id.GetStatus()),
				defaultDash(id.GetLang()),
				defaultDash(entry.GetOrigin()),
				defaultDash(entry.GetRelativePath()),
			)
		}
		_ = w.Flush()
	}

	if len(resp.GetPathBinaries()) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("PATH binaries:\n")
		for _, pathBinary := range resp.GetPathBinaries() {
			fmt.Fprintf(&b, "- %s\n", pathBinary)
		}
	}

	if b.Len() == 0 {
		return "No holons discovered."
	}
	return strings.TrimSpace(b.String())
}

func appendIdentityTable(b *strings.Builder, id *opv1.HolonIdentity) {
	if id == nil {
		return
	}

	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "FIELD\tVALUE")
	fmt.Fprintf(w, "UUID\t%s\n", defaultDash(id.GetUuid()))
	fmt.Fprintf(w, "Name\t%s\n", displayName(id))
	fmt.Fprintf(w, "Clade\t%s\n", cladeLabel(id.GetClade()))
	fmt.Fprintf(w, "Status\t%s\n", statusLabel(id.GetStatus()))
	fmt.Fprintf(w, "Lang\t%s\n", defaultDash(id.GetLang()))
	if len(id.GetAliases()) > 0 {
		fmt.Fprintf(w, "Aliases\t%s\n", strings.Join(id.GetAliases(), ", "))
	}
	_ = w.Flush()
}

func displayName(id *opv1.HolonIdentity) string {
	if id == nil {
		return "-"
	}

	parts := make([]string, 0, 2)
	if given := strings.TrimSpace(id.GetGivenName()); given != "" {
		parts = append(parts, given)
	}
	if family := strings.TrimSpace(id.GetFamilyName()); family != "" {
		parts = append(parts, family)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " ")
}

// identitySlug derives a lowercase-hyphenated slug from a proto identity.
func identitySlug(id *opv1.HolonIdentity) string {
	if id == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if g := strings.TrimSpace(id.GetGivenName()); g != "" {
		parts = append(parts, g)
	}
	if f := strings.TrimSpace(id.GetFamilyName()); f != "" {
		parts = append(parts, f)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(strings.Join(parts, "-"))
}

func cladeLabel(clade opv1.Clade) string {
	switch clade {
	case opv1.Clade_DETERMINISTIC_PURE:
		return "deterministic/pure"
	case opv1.Clade_DETERMINISTIC_STATEFUL:
		return "deterministic/stateful"
	case opv1.Clade_DETERMINISTIC_IO_BOUND:
		return "deterministic/io_bound"
	case opv1.Clade_PROBABILISTIC_GENERATIVE:
		return "probabilistic/generative"
	case opv1.Clade_PROBABILISTIC_PERCEPTUAL:
		return "probabilistic/perceptual"
	case opv1.Clade_PROBABILISTIC_ADAPTIVE:
		return "probabilistic/adaptive"
	default:
		return "-"
	}
}

func statusLabel(status opv1.Status) string {
	switch status {
	case opv1.Status_DRAFT:
		return "draft"
	case opv1.Status_STABLE:
		return "stable"
	case opv1.Status_DEPRECATED:
		return "deprecated"
	case opv1.Status_DEAD:
		return "dead"
	default:
		return "-"
	}
}

func shortUUID(uuid string) string {
	if len(uuid) > 8 {
		return uuid[:8]
	}
	if uuid == "" {
		return "-"
	}
	return uuid
}

func defaultDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func marshalProtoJSONForOutput(msg proto.Message) string {
	out, err := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}.Marshal(msg)
	if err != nil {
		return "{}"
	}
	return string(out)
}

func normalizeJSON(value string) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(value), "", "  "); err != nil {
		return value
	}
	return pretty.String()
}
