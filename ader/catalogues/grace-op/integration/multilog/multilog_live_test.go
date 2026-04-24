//go:build e2e

package multilog_test

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/observability"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMultilog_LiveRelayedChains(t *testing.T) {
	observability.Reset()
	defer observability.Reset()
	t.Setenv("OP_OBS", "logs,events")

	childOne := observability.Configure(observability.Config{Slug: "child-one", InstanceUID: "child-one-uid"})
	defer childOne.Close()
	childOne.Logger("child").Info("child one ready")
	childOne.Emit(context.Background(), observability.EventInstanceReady, map[string]string{"listener": "tcp://child-one"})
	childOneAddr, stopChildOne := serveObservability(t, childOne)
	defer stopChildOne()

	observability.Reset()
	childTwo := observability.Configure(observability.Config{Slug: "child-two", InstanceUID: "child-two-uid"})
	defer childTwo.Close()
	childTwo.Logger("child").Info("child two ready")
	childTwo.Emit(context.Background(), observability.EventInstanceReady, map[string]string{"listener": "tcp://child-two"})
	childTwoAddr, stopChildTwo := serveObservability(t, childTwo)
	defer stopChildTwo()

	observability.Reset()
	runDir := filepath.Join(t.TempDir(), "root")
	root := observability.Configure(observability.Config{
		Slug:         "root",
		InstanceUID:  "root-uid",
		OrganismUID:  "root-uid",
		OrganismSlug: "root",
		RunDir:       runDir,
	})
	defer root.Close()

	writer := observability.NewMultilogWriter(runDir)
	if err := writer.Start(); err != nil {
		t.Fatalf("start multilog writer: %v", err)
	}
	defer writer.Stop()

	relayOne, connOne := startRelay(t, "child-one", "child-one-uid", childOneAddr)
	defer connOne.Close()
	defer relayOne.Stop()
	relayTwo, connTwo := startRelay(t, "child-two", "child-two-uid", childTwoAddr)
	defer connTwo.Close()
	defer relayTwo.Stop()

	root.Logger("root").Info("root ready")
	root.Emit(context.Background(), observability.EventInstanceReady, map[string]string{"listener": "tcp://root"})

	multilogPath := filepath.Join(runDir, "multilog.jsonl")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		records, err := observability.ReadMultilog(multilogPath)
		if err == nil &&
			hasMultilogChain(records, "child-one", 2) &&
			hasMultilogChain(records, "child-two", 2) &&
			hasMultilogChain(records, "root", 1) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	records, _ := observability.ReadMultilog(multilogPath)
	t.Fatalf("multilog missing expected chain depths at %s: %+v", multilogPath, records)
}

func serveObservability(t *testing.T, obs *observability.Observability) (string, func()) {
	t.Helper()
	server := grpc.NewServer()
	holonsv1.RegisterHolonObservabilityServer(server, observability.NewService(obs, observability.VisibilityFull))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen child observability: %v", err)
	}
	go server.Serve(lis)
	return lis.Addr().String(), func() {
		server.Stop()
		_ = lis.Close()
	}
}

func startRelay(t *testing.T, slug, uid, address string) (*observability.Relay, *grpc.ClientConn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial child %s: %v", slug, err)
	}
	relay := observability.NewRelay(slug, uid, conn)
	if err := relay.Start(context.Background()); err != nil {
		_ = conn.Close()
		t.Fatalf("start relay %s: %v", slug, err)
	}
	return relay, conn
}

func hasMultilogChain(records []map[string]any, slug string, depth int) bool {
	for _, record := range records {
		if record["slug"] != slug {
			continue
		}
		chain, ok := record["chain"].([]any)
		if ok && len(chain) == depth {
			return true
		}
	}
	return false
}
