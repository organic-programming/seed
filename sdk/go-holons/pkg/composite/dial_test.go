package composite

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/observability"
	"google.golang.org/grpc"
)

type dialFakeServer struct {
	holonsv1.UnimplementedHolonMetaServer
	holonsv1.UnimplementedHolonObservabilityServer

	describeCount int32
	logsCount     int32
	eventsCount   int32
	followLogs    int32
	followEvents  int32
	closedLogs    int32
	closedEvents  int32
}

func (s *dialFakeServer) Describe(context.Context, *holonsv1.DescribeRequest) (*holonsv1.DescribeResponse, error) {
	atomic.AddInt32(&s.describeCount, 1)
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				GivenName:  "Peer",
				FamilyName: "Monitor",
				Aliases:    []string{"peer-monitor"},
			},
		},
	}, nil
}

func (s *dialFakeServer) Logs(req *holonsv1.LogsRequest, stream grpc.ServerStreamingServer[holonsv1.LogRecord]) error {
	atomic.AddInt32(&s.logsCount, 1)
	if !req.GetFollow() {
		now := time.Now()
		return stream.Send(&holonsv1.LogRecord{
			TimeUnixNano:         uint64(now.UnixNano()),
			ObservedTimeUnixNano: uint64(now.UnixNano()),
			SeverityNumber:       holonsv1.SeverityNumber_SEVERITY_NUMBER_INFO,
			SeverityText:         "INFO",
			Body:                 observability.ToAnyValue("ready"),
			Attributes:           peerAttrs(),
		})
	}
	atomic.AddInt32(&s.followLogs, 1)
	<-stream.Context().Done()
	atomic.AddInt32(&s.closedLogs, 1)
	return nil
}

func (s *dialFakeServer) Events(req *holonsv1.EventsRequest, stream grpc.ServerStreamingServer[holonsv1.LogRecord]) error {
	atomic.AddInt32(&s.eventsCount, 1)
	if !req.GetFollow() {
		now := time.Now()
		return stream.Send(&holonsv1.LogRecord{
			TimeUnixNano:         uint64(now.UnixNano()),
			ObservedTimeUnixNano: uint64(now.UnixNano()),
			SeverityNumber:       holonsv1.SeverityNumber_SEVERITY_NUMBER_INFO,
			SeverityText:         "INFO",
			Body:                 observability.ToAnyValue(observability.EventInstanceReady),
			Attributes:           peerAttrs(),
			EventName:            observability.EventInstanceReady,
		})
	}
	atomic.AddInt32(&s.followEvents, 1)
	<-stream.Context().Done()
	atomic.AddInt32(&s.closedEvents, 1)
	return nil
}

func (s *dialFakeServer) Metrics(*holonsv1.MetricsRequest, grpc.ServerStreamingServer[holonsv1.Metric]) error {
	return nil
}

func peerAttrs() []*holonsv1.KeyValue {
	return []*holonsv1.KeyValue{
		{Key: observability.AttrHolonsSlug, Value: observability.ToAnyValue("peer-monitor")},
		{Key: observability.AttrHolonsInstanceUID, Value: observability.ToAnyValue("peer-uid")},
	}
}

func TestDialParsesAddressFormsWithoutTransitiveRelay(t *testing.T) {
	t.Setenv("OP_OBS", "logs,events")
	observability.Reset()
	observability.Configure(observability.Config{Slug: "parent", InstanceUID: "parent-uid"})
	t.Cleanup(observability.Reset)

	cases := []struct {
		name    string
		network string
		address func(string) string
	}{
		{name: "bare tcp", network: "tcp", address: func(addr string) string { return addr }},
		{name: "tcp uri", network: "tcp", address: func(addr string) string { return "tcp://" + addr }},
		{name: "unix uri", network: "unix", address: func(addr string) string { return addr }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, address := startDialFakeServer(t, tc.network)
			conn, err := Dial(context.Background(), tc.address(address))
			if err != nil {
				t.Fatalf("Dial: %v", err)
			}
			if err := conn.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			if got := atomic.LoadInt32(&server.describeCount); got != 1 {
				t.Fatalf("Describe calls = %d, want 1", got)
			}
			if got := atomic.LoadInt32(&server.logsCount); got != 0 {
				t.Fatalf("Logs calls = %d, want 0", got)
			}
			if got := atomic.LoadInt32(&server.eventsCount); got != 0 {
				t.Fatalf("Events calls = %d, want 0", got)
			}
		})
	}
}

func TestDialRejectsStdioAddress(t *testing.T) {
	if _, err := Dial(context.Background(), "stdio://"); err == nil || !strings.Contains(err.Error(), "does not support stdio") {
		t.Fatalf("Dial(stdio) error = %v, want stdio rejection", err)
	}
}

func TestDialWithTransitiveObservabilityStartsRelayAndCloseStopsIt(t *testing.T) {
	t.Setenv("OP_OBS", "logs,events")
	observability.Reset()
	observability.Configure(observability.Config{Slug: "parent", InstanceUID: "parent-uid"})
	t.Cleanup(observability.Reset)

	server, address := startDialFakeServer(t, "tcp")
	conn, err := Dial(context.Background(), address, WithTransitiveObservability(true))
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	waitForAtomic(t, &server.followLogs, 1)
	waitForAtomic(t, &server.followEvents, 1)

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	waitForAtomic(t, &server.closedLogs, 1)
	waitForAtomic(t, &server.closedEvents, 1)
}

func startDialFakeServer(t *testing.T, network string) (*dialFakeServer, string) {
	t.Helper()
	server := &dialFakeServer{}
	grpcServer := grpc.NewServer()
	holonsv1.RegisterHolonMetaServer(grpcServer, server)
	holonsv1.RegisterHolonObservabilityServer(grpcServer, server)

	var (
		lis net.Listener
		err error
	)
	switch network {
	case "tcp":
		lis, err = net.Listen("tcp", "127.0.0.1:0")
	case "unix":
		path := filepath.Join(os.TempDir(), fmt.Sprintf("op-dial-%d.sock", time.Now().UnixNano()))
		t.Cleanup(func() { _ = os.Remove(path) })
		lis, err = net.Listen("unix", path)
	default:
		t.Fatalf("unsupported network %q", network)
	}
	if err != nil {
		t.Fatalf("listen %s: %v", network, err)
	}
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	if network == "unix" {
		return server, "unix://" + lis.Addr().String()
	}
	return server, lis.Addr().String()
}

func waitForAtomic(t *testing.T, value *int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(value) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("counter = %d, want at least %d", atomic.LoadInt32(value), want)
}
