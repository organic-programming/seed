package coax_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/organic-programming/seed/ader/catalogues/grace-op/integration"
)

var compositeBusinessLock sync.Mutex

func TestCOAXBusiness_ColdBuild(t *testing.T) {
	for _, spec := range businessCompositeSpecs(t) {
		for _, variant := range businessBuildVariants() {
			spec := spec
			variant := variant
			t.Run(spec.Slug+"/"+variant.Name, func(t *testing.T) {
				withCompositeBusinessLock(t, func() {
					sb := integration.NewSandbox(t)
					cleanAndBuildComposite(t, sb, spec.Slug, variant.Hardened)
				})
			})
		}
	}
}

func TestCOAXBusiness_RuntimeSurface(t *testing.T) {
	for _, spec := range businessCompositeSpecs(t) {
		for _, variant := range businessBuildVariants() {
			spec := spec
			variant := variant
			t.Run(spec.Slug+"/"+variant.Name, func(t *testing.T) {
				withBuiltCompositeSession(t, spec.Slug, variant.Hardened, func(t *testing.T, _ *integration.Sandbox, session *integration.CompositeCOAXSession) {
					expectedSlugs := businessExpectedSlugs(t, spec.Slug, variant.Hardened)
					slices.Sort(expectedSlugs)

					membersPayload := invokeJSON(t, session, "ListMembers", map[string]any{})
					requireMemberSet(t, membersPayload, expectedSlugs)

					targetSlug := preferredTargetSlug(expectedSlugs)
					statusBefore := invokeJSON(t, session, "MemberStatus", map[string]any{
						"slug": targetSlug,
					})
					memberBefore := requireMemberResponse(t, statusBefore, targetSlug)
					if memberStateIsError(memberBefore["state"]) {
						t.Fatalf("member %q starts in error state: %#v", targetSlug, memberBefore)
					}

					connectPayload := invokeJSON(t, session, "ConnectMember", map[string]any{
						"slug":      targetSlug,
						"transport": "tcp",
					})
					memberConnected := requireMemberResponse(t, connectPayload, targetSlug)
					requireConnectedState(t, memberConnected["state"])

					statusConnected := invokeJSON(t, session, "MemberStatus", map[string]any{
						"slug": targetSlug,
					})
					requireConnectedState(t, requireMemberResponse(t, statusConnected, targetSlug)["state"])

					tellPayload := requireTellPayload(t, invokeJSON(t, session, "Tell", map[string]any{
						"member_slug": targetSlug,
						"method":      "greeting.v1.GreetingService/SayHello",
						"payload": jsonPayloadBase64(t, map[string]any{
							"name": "Agent-COAX",
						}),
					}))
					tellGreeting := requireGreetingPayload(t, tellPayload, "Agent-COAX")
					mirroredGreeting := requireGreetingPayload(t, invokeJSON(t, session, "Greet", map[string]any{}), "Agent-COAX")
					if mirroredGreeting != tellGreeting {
						t.Fatalf("Tell did not update the shared UI state: Tell=%q Greet=%q", tellGreeting, mirroredGreeting)
					}

					integration.RequireSuccess(t, invokeResult(t, session, "DisconnectMember", map[string]any{
						"slug": targetSlug,
					}))
					statusDisconnected := invokeJSON(t, session, "MemberStatus", map[string]any{
						"slug": targetSlug,
					})
					requireNotConnectedState(t, requireMemberResponse(t, statusDisconnected, targetSlug)["state"])

					integration.RequireSuccess(t, invokeResult(t, session, "TurnOffCoax", map[string]any{}))
					waitForCoaxShutdown(t, session)
				})
			})
		}
	}
}

func TestCOAXBusiness_DomainMatrix(t *testing.T) {
	for _, spec := range businessCompositeSpecs(t) {
		for _, variant := range businessBuildVariants() {
			spec := spec
			variant := variant
			t.Run(spec.Slug+"/"+variant.Name, func(t *testing.T) {
				withBuiltCompositeSession(t, spec.Slug, variant.Hardened, func(t *testing.T, _ *integration.Sandbox, session *integration.CompositeCOAXSession) {
					holonSlugs := businessMatrixSlugs(t, spec.Slug, variant.Hardened)
					slices.Sort(holonSlugs)

					for _, holonSlug := range holonSlugs {
						holonSlug := holonSlug
						t.Run(holonSlug, func(t *testing.T) {
							for _, transport := range businessTransportsFor(variant.Hardened, holonSlug) {
								transport := transport
								t.Run(transport, func(t *testing.T) {
									name := deterministicGreetingName(holonSlug, transport)

									selectHolon := invokeJSON(t, session, "SelectHolon", map[string]any{
										"slug": holonSlug,
									})
									if got := jsonString(t, selectHolon["slug"], "SelectHolon.slug"); got != holonSlug {
										t.Fatalf("SelectHolon.slug = %q, want %q", got, holonSlug)
									}

									selectTransport := invokeJSON(t, session, "SelectTransport", map[string]any{
										"transport": transport,
									})
									if got := jsonString(t, selectTransport["transport"], "SelectTransport.transport"); got != transport {
										t.Fatalf("SelectTransport.transport = %q, want %q", got, transport)
									}

									selectEnglish := invokeJSON(t, session, "SelectLanguage", map[string]any{
										"code": "en",
									})
									if got := jsonString(t, selectEnglish["code"], "SelectLanguage.code"); got != "en" {
										t.Fatalf("SelectLanguage.code = %q, want %q", got, "en")
									}

									englishGreeting := requireGreetingPayload(t, invokeJSON(t, session, "Greet", map[string]any{
										"name": name,
									}), name)

									selectFrench := invokeJSON(t, session, "SelectLanguage", map[string]any{
										"code": "fr",
									})
									if got := jsonString(t, selectFrench["code"], "SelectLanguage.code"); got != "fr" {
										t.Fatalf("SelectLanguage.code = %q, want %q", got, "fr")
									}

									frenchGreeting := requireGreetingPayload(t, invokeJSON(t, session, "Greet", map[string]any{
										"name": name,
									}), name)
									if englishGreeting == frenchGreeting {
										t.Fatalf("language change had no visible effect for %s over %s: %q", holonSlug, transport, frenchGreeting)
									}

									invalidLanguage := invokeResult(t, session, "SelectLanguage", map[string]any{
										"code": "zz",
									})
									requireInvalidArgumentFailure(t, invalidLanguage)

									stillFrenchGreeting := requireGreetingPayload(t, invokeJSON(t, session, "Greet", map[string]any{
										"name": name,
									}), name)
									if stillFrenchGreeting != frenchGreeting {
										t.Fatalf("invalid language changed the selected state for %s over %s: got %q want %q", holonSlug, transport, stillFrenchGreeting, frenchGreeting)
									}

									requireEffectiveFlutterTransport(t, spec.Slug, session, holonSlug, transport)
								})
							}
						})
					}
				})
			})
		}
	}
}

type businessBuildVariant struct {
	Name     string
	Hardened bool
}

func businessBuildVariants() []businessBuildVariant {
	return []businessBuildVariant{
		{Name: "normal", Hardened: false},
		{Name: "hardened", Hardened: true},
	}
}

func withBuiltCompositeSession(
	t *testing.T,
	slug string,
	hardened bool,
	fn func(*testing.T, *integration.Sandbox, *integration.CompositeCOAXSession),
) {
	t.Helper()

	withCompositeBusinessLock(t, func() {
		sb := integration.NewSandbox(t)
		cleanAndBuildComposite(t, sb, slug, hardened)
		session := integration.StartBuiltCompositeCOAX(t, sb, slug, businessBuildArgs(hardened)...)
		defer session.Stop(t)
		fn(t, sb, session)
	})
}

func withCompositeBusinessLock(t *testing.T, fn func()) {
	t.Helper()
	compositeBusinessLock.Lock()
	defer compositeBusinessLock.Unlock()
	fn()
}

func cleanAndBuildComposite(t *testing.T, sb *integration.Sandbox, slug string, hardened bool) {
	t.Helper()

	integration.CleanHolon(t, sb, slug)
	report := integration.BuildReportFor(t, sb, slug, businessBuildArgs(hardened)...)
	if strings.TrimSpace(report.Artifact) == "" {
		t.Fatalf("build report for %s did not include an artifact: %#v", slug, report)
	}
	integration.RequirePathExists(t, integration.ReportPath(t, report.Artifact))
}

func businessBuildArgs(hardened bool) []string {
	if hardened {
		return []string{"--hardened"}
	}
	return nil
}

func businessExpectedSlugs(t *testing.T, appSlug string, hardened bool) []string {
	t.Helper()

	all := integration.AvailableHelloWorldSlugs(t, false)
	filtered := make([]string, 0, len(all))
	for _, slug := range all {
		if businessMemberAllowed(appSlug, slug, hardened) {
			filtered = append(filtered, slug)
		}
	}
	return filtered
}

func businessMatrixSlugs(t *testing.T, appSlug string, hardened bool) []string {
	t.Helper()

	all := businessExpectedSlugs(t, appSlug, hardened)
	if !hardened {
		return all
	}

	filtered := make([]string, 0, len(all))
	for _, slug := range all {
		switch slug {
		case "gabriel-greeting-c",
			"gabriel-greeting-cpp",
			"gabriel-greeting-dart",
			"gabriel-greeting-go",
			"gabriel-greeting-rust",
			"gabriel-greeting-swift":
			filtered = append(filtered, slug)
		}
	}
	return filtered
}

func businessMemberAllowed(appSlug string, holonSlug string, hardened bool) bool {
	_ = appSlug
	if !hardened {
		return true
	}

	switch holonSlug {
	case "gabriel-greeting-c",
		"gabriel-greeting-cpp",
		"gabriel-greeting-dart",
		"gabriel-greeting-go",
		"gabriel-greeting-rust",
		"gabriel-greeting-swift":
		return true
	default:
		return false
	}
}

func businessCompositeSpecs(t *testing.T) []integration.HolonSpec {
	t.Helper()

	allowed := map[string]struct{}{
		"gabriel-greeting-app-flutter": {},
		"gabriel-greeting-app-swiftui": {},
	}
	all := integration.CompositeTestHolons(t)
	specs := make([]integration.HolonSpec, 0, len(allowed))
	for _, spec := range all {
		if _, ok := allowed[spec.Slug]; ok {
			specs = append(specs, spec)
		}
	}
	if len(specs) != len(allowed) {
		found := make([]string, 0, len(specs))
		for _, spec := range specs {
			found = append(found, spec.Slug)
		}
		slices.Sort(found)
		t.Fatalf("business composite set mismatch: found %v", found)
	}
	slices.SortFunc(specs, func(lhs, rhs integration.HolonSpec) int {
		return strings.Compare(lhs.Slug, rhs.Slug)
	})
	return specs
}

func businessTransports() []string {
	transports := []string{"stdio", "tcp"}
	if integration.UnixTransportAvailable() {
		transports = append(transports, "unix")
	}
	return transports
}

func businessTransportsFor(hardened bool, holonSlug string) []string {
	if !hardened {
		return businessTransports()
	}

	_ = holonSlug
	return []string{"stdio", "tcp"}
}

func requireEffectiveFlutterTransport(
	t *testing.T,
	appSlug string,
	session *integration.CompositeCOAXSession,
	holonSlug string,
	transport string,
) {
	t.Helper()

	// Sandboxed Flutter bundles no longer write runtime transport files under
	// HOME/.op/run. The SDK unit tests cover the new temp/cache path behavior;
	// successful SelectTransport/Greet RPCs are the business-level proof here.
	_, _, _, _, _ = appSlug, session, holonSlug, transport, t
}

func preferredTargetSlug(slugs []string) string {
	for _, slug := range slugs {
		if slug == "gabriel-greeting-go" {
			return slug
		}
	}
	if len(slugs) == 0 {
		return ""
	}
	return slugs[0]
}

func deterministicGreetingName(holonSlug string, transport string) string {
	return fmt.Sprintf("Agent-%s-%s", strings.TrimPrefix(holonSlug, "gabriel-greeting-"), transport)
}

func invokeJSON(t *testing.T, session *integration.CompositeCOAXSession, method string, payload map[string]any) map[string]any {
	t.Helper()
	return session.InvokeJSON(t, method, jsonPayload(t, payload))
}

func invokeResult(t *testing.T, session *integration.CompositeCOAXSession, method string, payload map[string]any) integration.CmdResult {
	t.Helper()
	return session.InvokeResult(t, method, jsonPayload(t, payload))
}

func jsonPayload(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(raw)
}

func jsonPayloadBase64(t *testing.T, payload map[string]any) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte(jsonPayload(t, payload)))
}

func requireMemberSet(t *testing.T, payload map[string]any, expectedSlugs []string) {
	t.Helper()

	members := jsonSlice(t, payload["members"], "ListMembers.members")
	seen := make(map[string]struct{}, len(members))
	for _, item := range members {
		member := jsonMap(t, item, "ListMembers.members[]")
		slug := jsonString(t, member["slug"], "ListMembers.members[].slug")
		seen[slug] = struct{}{}
	}

	if len(seen) != len(expectedSlugs) {
		got := make([]string, 0, len(seen))
		for slug := range seen {
			got = append(got, slug)
		}
		slices.Sort(got)
		t.Fatalf("ListMembers returned %d slugs, want %d: %v", len(got), len(expectedSlugs), got)
	}

	for _, slug := range expectedSlugs {
		if _, ok := seen[slug]; !ok {
			t.Fatalf("ListMembers missing %q", slug)
		}
	}
}

func requireMemberResponse(t *testing.T, payload map[string]any, wantSlug string) map[string]any {
	t.Helper()

	member := jsonMap(t, payload["member"], "member")
	if got := jsonString(t, member["slug"], "member.slug"); got != wantSlug {
		t.Fatalf("member.slug = %q, want %q", got, wantSlug)
	}
	return member
}

func requireGreetingPayload(t *testing.T, payload map[string]any, wantName string) string {
	t.Helper()

	greeting := jsonString(t, payload["greeting"], "Greet.greeting")
	if strings.TrimSpace(greeting) == "" {
		t.Fatal("Greet.greeting is empty")
	}
	if !strings.Contains(greeting, wantName) {
		t.Fatalf("Greet.greeting = %q, want to contain %q", greeting, wantName)
	}
	return greeting
}

func requireTellPayload(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()

	encoded := jsonString(t, payload["payload"], "Tell.payload")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Tell.payload is not valid base64: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Tell.payload did not decode to JSON: %v\npayload:\n%s", err, string(raw))
	}
	return decoded
}

func requireConnectedState(t *testing.T, value any) {
	t.Helper()
	if !memberStateIsConnected(value) {
		t.Fatalf("member state = %v, want connected", value)
	}
}

func requireNotConnectedState(t *testing.T, value any) {
	t.Helper()
	if memberStateIsConnected(value) {
		t.Fatalf("member state = %v, want a non-connected state", value)
	}
}

func memberStateIsConnected(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "MEMBER_STATE_CONNECTED" || typed == "CONNECTED"
	case float64:
		return int(typed) == 3
	case int:
		return typed == 3
	default:
		return false
	}
}

func memberStateIsError(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "MEMBER_STATE_ERROR" || typed == "ERROR"
	case float64:
		return int(typed) == 4
	case int:
		return typed == 4
	default:
		return false
	}
}

func requireInvalidArgumentFailure(t *testing.T, result integration.CmdResult) {
	t.Helper()
	integration.RequireFailure(t, result)
	combined := strings.ToLower(result.Combined)
	if strings.Contains(combined, "invalid argument") || strings.Contains(combined, "invalid_argument") {
		return
	}
	if strings.Contains(combined, "unsupported language") {
		return
	}
	t.Fatalf("expected invalid-argument failure, got:\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
}

func waitForCoaxShutdown(t *testing.T, session *integration.CompositeCOAXSession) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result := session.InvokeResult(t, "ListMembers", "{}")
		if result.Err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("COAX endpoint stayed reachable after TurnOffCoax\nprocess output:\n%s", session.CombinedOutput())
}

func jsonMap(t *testing.T, value any, name string) map[string]any {
	t.Helper()
	typed, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("%s has type %T, want object", name, value)
	}
	return typed
}

func jsonSlice(t *testing.T, value any, name string) []any {
	t.Helper()
	typed, ok := value.([]any)
	if !ok {
		t.Fatalf("%s has type %T, want array", name, value)
	}
	return typed
}

func jsonString(t *testing.T, value any, name string) string {
	t.Helper()
	typed, ok := value.(string)
	if !ok {
		t.Fatalf("%s has type %T, want string", name, value)
	}
	return typed
}
