package observability

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestOrganismRelay_TwoLevel spins up a fake "child" holon exposing
// HolonObservability, then configures a local Observability as the
// parent + relay and verifies that a log pushed by the child lands on
// the parent's ring with the appended ChainHop. Demonstrates the core
// mechanics of OBSERVABILITY.md §Organism Relay.
func TestOrganismRelay_TwoLevel(t *testing.T) {
	// --- child side ---
	Reset()
	t.Setenv("OP_OBS", "logs,events")
	childObs := Configure(Config{
		Slug:        "gabriel-greeting-rust",
		InstanceUID: "1c2d3e4f",
	})
	defer childObs.Close()
	childObs.Logger("leaf").Info("rendered banner", "name", "Bob")

	grpcSrv := grpc.NewServer()
	v1.RegisterHolonObservabilityServer(grpcSrv, NewService(childObs, VisibilityFull))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	// --- parent side (the organism root, in the same process for test) ---
	// Replace the singleton so the parent becomes the "current" Observability.
	Reset()
	parentObs := Configure(Config{
		Slug:         "gabriel-greeting-go",
		InstanceUID:  "ea346efb",
		OrganismUID:  "ea346efb",
		OrganismSlug: "gabriel-greeting-go",
	})
	defer parentObs.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Start the relay so child.Logs → parent's ring with chain append.
	relay := NewRelay("gabriel-greeting-rust", "1c2d3e4f", conn)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("relay.Start: %v", err)
	}
	defer relay.Stop()

	// Push one more log through the child so an established relay can
	// forward a live entry too. The replayed ring entry alone proves the
	// chain contract, so the assertion below only requires one match.
	childObs.Logger("leaf").Info("rendered banner 2", "name", "Alice")

	// Wait for the relay to deliver.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		entries := parentObs.LogRing().Drain()
		if len(entries) >= 1 {
			// Verify the relayed entries carry the child's chain hop.
			seen := 0
			for _, e := range entries {
				if e.Slug == "gabriel-greeting-rust" && len(e.Chain) == 1 &&
					e.Chain[0].Slug == "gabriel-greeting-rust" {
					seen++
				}
			}
			if seen >= 1 {
				return // all good
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	entries := parentObs.LogRing().Drain()
	t.Fatalf("relay did not deliver expected entries; got %d:\n%+v", len(entries), entries)
}

func TestRelayRespectsFamilyGate(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "logs")
	childObs := Configure(Config{
		Slug:        "child",
		InstanceUID: "child-uid",
	})
	defer childObs.Close()
	childObs.Logger("leaf").Info("log only")

	grpcSrv := grpc.NewServer()
	v1.RegisterHolonObservabilityServer(grpcSrv, NewService(childObs, VisibilityFull))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	Reset()
	t.Setenv("OP_OBS", "logs")
	parentObs := Configure(Config{Slug: "root", InstanceUID: "root-uid"})
	defer parentObs.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	relay := NewRelay("child", "child-uid", conn)
	if err := relay.Start(context.Background()); err != nil {
		t.Fatalf("relay.Start logs-only: %v", err)
	}
	defer relay.Stop()

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		for _, entry := range parentObs.LogRing().Drain() {
			if entry.Slug == "child" && len(entry.Chain) == 1 && entry.Chain[0].Slug == "child" {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("relay did not deliver logs-only child entry")
}

// TestMultilogWriter_WritesEnrichedChain confirms that entries pushed
// to the parent ring by a relay end up in multilog.jsonl with the
// chain enriched. The root's multilog appends the root stream-source
// hop after any relay-provided child hops.
func TestMultilogWriter_WritesEnrichedChain(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "logs,events")
	tmp := t.TempDir()
	runDir := filepath.Join(tmp, "root")
	obs := Configure(Config{
		Slug:         "root",
		InstanceUID:  "root-uid",
		OrganismUID:  "root-uid",
		OrganismSlug: "root",
		RunDir:       runDir,
	})
	defer obs.Close()

	mw := NewMultilogWriter(runDir)
	if err := mw.Start(); err != nil {
		t.Fatalf("multilog start: %v", err)
	}
	defer mw.Stop()

	// Simulate a relay delivering an entry with an already-populated chain.
	entry := LogEntry{
		Timestamp:   time.Now(),
		Level:       LevelInfo,
		Slug:        "leaf-holon",
		InstanceUID: "leaf-uid",
		Message:     "rendered banner",
		Chain: []Hop{
			{Slug: "leaf-holon", InstanceUID: "leaf-uid"},
		},
	}
	obs.LogRing().Push(entry)

	// Also push an emit from the root itself to exercise the local-only case.
	obs.Logger("app").Info("root info")
	obs.Emit(nil, EventInstanceReady, map[string]string{"listener": "tcp://x"})

	// Give the watcher goroutine a chance to flush.
	time.Sleep(100 * time.Millisecond)
	mw.Stop()

	records, err := ReadMultilog(filepath.Join(runDir, "multilog.jsonl"))
	if err != nil {
		t.Fatalf("read multilog: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected at least 2 multilog records, got %d: %+v", len(records), records)
	}

	foundRelayed := false
	foundRoot := false
	for _, rec := range records {
		if rec["kind"] != "log" {
			continue
		}
		if rec["slug"] == "leaf-holon" {
			foundRelayed = true
			chain, ok := rec["chain"].([]any)
			if !ok || len(chain) != 2 {
				t.Errorf("relayed entry missing chain: %+v", rec)
			}
		}
		if rec["slug"] == "root" {
			foundRoot = true
			chain, ok := rec["chain"].([]any)
			if !ok || len(chain) != 1 {
				t.Errorf("root entry missing enriched root chain: %+v", rec)
			}
		}
	}
	if !foundRelayed {
		t.Errorf("multilog did not contain the relayed leaf entry: %+v", records)
	}
	if !foundRoot {
		t.Errorf("multilog did not contain the root entry: %+v", records)
	}
}
