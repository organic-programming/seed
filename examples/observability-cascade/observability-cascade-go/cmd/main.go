package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/organic-programming/go-holons/pkg/composite"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/observability"
	"github.com/organic-programming/go-holons/pkg/serve"
	"google.golang.org/grpc"
	relayv1 "observability-cascade-go-node/gen/go/relay/v1"
	"observability-cascade-go/gen"
	ocv1 "observability-cascade-go/gen/go/observability_cascade/v1"
)

const (
	goSlug   = "observability-cascade-go-node"
	dartSlug = "observability-cascade-dart-node"
	runTicks = 3
)

type cascadeService struct {
	ocv1.UnimplementedObservabilityCascadeServiceServer
}

type languageMember struct {
	lang   string
	slug   string
	binary string
}

func init() {
	describe.UseStaticResponse(gen.StaticDescribeResponse())
}

func main() {
	if len(os.Args) > 1 && canonicalCommand(os.Args[1]) == "serve" {
		options := serve.ParseOptions(os.Args[2:])
		if err := serve.RunCLIOptions(options, func(s *grpc.Server) {
			ocv1.RegisterObservabilityCascadeServiceServer(s, &cascadeService{})
		}); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
		return
	}

	live := flag.Bool("live-stream", false, "run live-stream cascade validation")
	multi := flag.Bool("multi-pattern", false, "run mixed-language cascade validation")
	flag.Parse()

	var failed int32
	switch {
	case *multi:
		report := runMultiPatternReport(true)
		failed = report.GetTotalFail()
	default:
		name := "default"
		liveMode := false
		if *live {
			name = "live-stream"
			liveMode = true
		}
		report := runReport(name, ownLanguageMembers(), liveMode, true)
		failed = report.GetFail()
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func (s *cascadeService) RunDefault(context.Context, *ocv1.RunRequest) (*ocv1.CascadeReport, error) {
	return runReport("default", ownLanguageMembers(), false, false), nil
}

func (s *cascadeService) RunLiveStream(context.Context, *ocv1.RunRequest) (*ocv1.CascadeReport, error) {
	return runReport("live-stream", ownLanguageMembers(), true, false), nil
}

func (s *cascadeService) RunMultiPattern(context.Context, *ocv1.RunRequest) (*ocv1.MultiPatternReport, error) {
	return runMultiPatternReport(false), nil
}

func runMultiPatternReport(emit bool) *ocv1.MultiPatternReport {
	totalStart := time.Now()
	patterns := goPatterns()
	out := &ocv1.MultiPatternReport{}
	if emit {
		fmt.Println("=== observability-cascade-go --multi-pattern ===")
		fmt.Println()
	}
	for i, pattern := range patterns {
		if emit {
			fmt.Printf("Pattern %d/%d: %s\n", i+1, len(patterns), pattern.name)
		}
		report := runReport(pattern.name, pattern.members, true, emit)
		out.Patterns = append(out.Patterns, report)
		out.TotalPass += report.GetPass()
		out.TotalFail += report.GetFail()
		if emit {
			status := "PASS"
			if report.GetFail() > 0 {
				status = "FAIL"
			}
			fmt.Printf("Pattern %s: %d/%d %s (elapsed=%s)\n", pattern.name, report.GetPass(), report.GetTicks(), status, elapsedText(report.GetElapsedUs()))
			fmt.Println()
		}
	}
	out.TotalElapsedUs = time.Since(totalStart).Microseconds()
	if emit {
		fmt.Printf("Summary: %d PASS / %d FAIL across %d ticks (total elapsed=%s)\n", out.TotalPass, out.TotalFail, out.TotalPass+out.TotalFail, elapsedText(out.TotalElapsedUs))
	}
	return out
}

func runReport(name string, members []languageMember, live bool, emit bool) *ocv1.CascadeReport {
	ensureCascadeObservability()
	reportStart := time.Now()
	report := &ocv1.CascadeReport{Name: name}
	poll := 100 * time.Millisecond
	timeout := 3 * time.Second
	if live {
		poll = 50 * time.Millisecond
		timeout = time.Second
	}
	if emit {
		fmt.Printf("=== observability-cascade-go %s===\n\n", modeSuffix(name))
	}

	for phaseIdx, transportName := range composite.TransportCoverageSequence {
		phaseStart := time.Now()
		from := transportName
		if phaseIdx > 0 {
			from = composite.TransportCoverageSequence[phaseIdx-1]
		}
		phase := &ocv1.PhaseResult{Name: fmt.Sprintf("%02d-%s→%s", phaseIdx+1, from, transportName)}
		if emit {
			fmt.Printf("Phase %d/%d: %s\n", phaseIdx+1, len(composite.TransportCoverageSequence), phase.Name)
		}
		specs := childSpecs(members)
		cascade, err := composite.BuildCascade(context.Background(), composite.CascadeOptions{
			Transport: transportName,
			Members:   specs,
			ExtraEnv: map[string]string{
				"OP_OBS":       "logs,events,metrics,prom",
				"OP_PROM_ADDR": "127.0.0.1:0",
			},
		})
		if err != nil {
			phase.Fail += runTicks
			for tick := 1; tick <= runTicks; tick++ {
				phase.Failures = append(phase.Failures, fmt.Sprintf("tick=%d log=%s event=%s hops=%s", tick, "spawn", "spawn", compactEvidence(err.Error())))
			}
			phase.ElapsedUs = time.Since(phaseStart).Microseconds()
			addPhase(report, phase)
			if emit {
				printPhaseSummary(phase)
			}
			continue
		}

		previous := map[string]int64{}
		for tick := 1; tick <= runTicks; tick++ {
			sender := fmt.Sprintf("%s-phase-%02d-tick-%d", name, phaseIdx+1, tick)
			result := runTick(cascade, sender, transportName, members, previous, timeout, poll, live)
			if result.pass {
				phase.Pass++
			} else {
				phase.Fail++
				phase.Failures = append(phase.Failures, result.evidenceLine(tick))
			}
			if emit {
				fmt.Printf("  Tick %d/%d: %s\n", tick, runTicks, passText(result.pass))
				if !result.pass {
					fmt.Fprintln(os.Stderr, "    "+result.evidenceLine(tick))
				}
			}
		}
		_ = cascade.Stop(context.Background())
		phase.ElapsedUs = time.Since(phaseStart).Microseconds()
		addPhase(report, phase)
		if emit {
			printPhaseSummary(phase)
		}
	}
	report.ElapsedUs = time.Since(reportStart).Microseconds()
	if emit {
		fmt.Printf("\nSummary: %d ticks, %d PASS, %d FAIL (total elapsed=%s)\n", report.Ticks, report.Pass, report.Fail, elapsedText(report.ElapsedUs))
	}
	return report
}

type tickResult struct {
	pass  bool
	log   composite.CheckOutcome
	event composite.CheckOutcome
	hops  composite.CheckOutcome
}

func (r tickResult) evidenceLine(tick int) string {
	return fmt.Sprintf("tick=%d log=%s event=%s hops=%s", tick, evidenceText(r.log), evidenceText(r.event), evidenceText(r.hops))
}

func runTick(cascade *composite.Cascade, sender, note string, members []languageMember, previous map[string]int64, timeout, poll time.Duration, live bool) tickResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := relayv1.NewRelayServiceClient(cascade.Top.Conn).Tick(ctx, &relayv1.TickRequest{Sender: sender, Note: note})
	cancel()
	if err != nil {
		out := composite.CheckOutcome{Evidence: compactEvidence(err.Error())}
		return tickResult{log: out, event: out, hops: out}
	}
	hops := checkHops(resp.GetHops(), members, previous)
	if !hops.Pass {
		return tickResult{hops: hops, log: composite.CheckOutcome{Evidence: "skipped"}, event: composite.CheckOutcome{Evidence: "skipped"}}
	}
	expected := hopChain(resp.GetHops())
	leafUID := resp.GetHops()[0].GetUid()
	log := composite.CheckRelayedLog(context.Background(), composite.LogCheckOptions{
		Sender:        sender,
		LeafUID:       leafUID,
		ExpectedChain: expected,
		Timeout:       timeout,
		PollInterval:  poll,
		Live:          live,
	})
	event := composite.CheckRelayedEvent(context.Background(), composite.EventCheckOptions{
		EventName:     observability.EventInstanceReady,
		LeafUID:       leafUID,
		ExpectedChain: expected,
		Timeout:       timeout,
		PollInterval:  poll,
		Live:          live,
	})
	return tickResult{pass: hops.Pass && log.Pass && event.Pass, hops: hops, log: log, event: event}
}

func checkHops(hops []*relayv1.HopReceipt, members []languageMember, previous map[string]int64) composite.CheckOutcome {
	if len(hops) != len(members) {
		return composite.CheckOutcome{Evidence: fmt.Sprintf("hops length %d want %d", len(hops), len(members))}
	}
	for i, hop := range hops {
		want := members[len(members)-1-i]
		if hop.GetSlug() != want.slug {
			return composite.CheckOutcome{Evidence: fmt.Sprintf("hop %d slug=%s want %s", i, hop.GetSlug(), want.slug)}
		}
		if hop.GetUid() == "" {
			return composite.CheckOutcome{Evidence: fmt.Sprintf("hop %d uid empty", i)}
		}
		if hop.GetReceived() <= previous[hop.GetUid()] {
			return composite.CheckOutcome{Evidence: fmt.Sprintf("hop %d received=%d previous=%d", i, hop.GetReceived(), previous[hop.GetUid()])}
		}
		previous[hop.GetUid()] = hop.GetReceived()
	}
	return composite.CheckOutcome{Pass: true}
}

func hopChain(hops []*relayv1.HopReceipt) []string {
	out := make([]string, 0, len(hops))
	for _, hop := range hops {
		out = append(out, hop.GetSlug())
	}
	return out
}

func ownLanguageMembers() []languageMember {
	binary, err := composite.Member("go-node")
	if err != nil {
		return []languageMember{{lang: "go", slug: goSlug, binary: ""}}
	}
	return []languageMember{
		{lang: "go", slug: goSlug, binary: binary},
		{lang: "go", slug: goSlug, binary: binary},
		{lang: "go", slug: goSlug, binary: binary},
	}
}

type namedPattern struct {
	name    string
	members []languageMember
}

func goPatterns() []namedPattern {
	goBin, _ := composite.Member("go-node")
	dartBin, _ := composite.Member("dart-node")
	bins := map[string]languageMember{
		"go":   {lang: "go", slug: goSlug, binary: goBin},
		"dart": {lang: "dart", slug: dartSlug, binary: dartBin},
	}
	names := []string{
		"go-go-go", "go-go-dart", "go-dart-go", "go-dart-dart",
		"dart-go-go", "dart-go-dart", "dart-dart-go", "dart-dart-dart",
	}
	out := make([]namedPattern, 0, len(names))
	for _, name := range names {
		parts := strings.Split(name, "-")
		out = append(out, namedPattern{
			name: name,
			members: []languageMember{
				bins[parts[0]],
				bins[parts[1]],
				bins[parts[2]],
			},
		})
	}
	return out
}

func childSpecs(members []languageMember) []composite.ChildSpec {
	out := make([]composite.ChildSpec, 0, len(members))
	for _, member := range members {
		out = append(out, composite.ChildSpec{Slug: member.slug, Binary: member.binary})
	}
	return out
}

func addPhase(report *ocv1.CascadeReport, phase *ocv1.PhaseResult) {
	report.Phases = append(report.Phases, phase)
	report.Pass += phase.Pass
	report.Fail += phase.Fail
	report.Ticks += phase.Pass + phase.Fail
}

func ensureCascadeObservability() {
	obs := observability.Current()
	if obs.Enabled(observability.FamilyLogs) && obs.Enabled(observability.FamilyEvents) {
		return
	}
	_ = os.Setenv("OP_OBS", "logs,events,metrics,prom")
	observability.FromEnv(observability.Config{Slug: "observability-cascade-go"})
}

func evidenceText(out composite.CheckOutcome) string {
	if out.Pass {
		return "ok"
	}
	return compactEvidence(out.Evidence)
}

func compactEvidence(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "<empty>"
	}
	if len(value) <= 240 {
		return value
	}
	return value[:240] + "..."
}

func passText(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

func printPhaseSummary(phase *ocv1.PhaseResult) {
	status := "PASS"
	if phase.GetFail() > 0 {
		status = "FAIL"
	}
	fmt.Printf("Phase %s: %d/%d %s (elapsed=%s)\n", phase.GetName(), phase.GetPass(), phase.GetPass()+phase.GetFail(), status, elapsedText(phase.GetElapsedUs()))
}

func elapsedText(elapsedUS int64) string {
	d := time.Duration(elapsedUS) * time.Microsecond
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	default:
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
}

func modeSuffix(name string) string {
	if name == "default" {
		return ""
	}
	return "--" + name + " "
}

func canonicalCommand(raw string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}
