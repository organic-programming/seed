package observability

import (
	"context"
	"testing"
	"time"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc/metadata"
)

func TestEventsFollowReplaysRingOnSubscribe(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "events")
	obs := Configure(Config{Slug: "server-test", InstanceUID: "uid-events"})
	defer obs.Close()

	obs.Emit(context.Background(), EventInstanceReady, map[string]string{"phase": "replay"})
	ctx, cancel := context.WithCancel(context.Background())
	stream := &captureEventsStream{
		ctx:   ctx,
		sends: make(chan *v1.EventInfo, 4),
		onSend: func(count int) {
			if count == 1 {
				obs.Emit(context.Background(), EventInstanceExited, map[string]string{"phase": "live"})
			}
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- NewService(obs, VisibilityFull).Events(&v1.EventsRequest{Follow: true}, stream)
	}()

	first := recvEvent(t, stream.sends)
	if first.GetType() != v1.EventType_INSTANCE_READY || first.GetPayload()["phase"] != "replay" {
		t.Fatalf("first event = %+v, want replay INSTANCE_READY", first)
	}
	second := recvEvent(t, stream.sends)
	if second.GetType() != v1.EventType_INSTANCE_EXITED || second.GetPayload()["phase"] != "live" {
		t.Fatalf("second event = %+v, want live INSTANCE_EXITED", second)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Events returned error: %v", err)
	}
}

func TestLogsFollowReplaysRingOnSubscribe(t *testing.T) {
	Reset()
	t.Setenv("OP_OBS", "logs")
	obs := Configure(Config{Slug: "server-test", InstanceUID: "uid-logs"})
	defer obs.Close()

	obs.Logger("test").Info("replay")
	ctx, cancel := context.WithCancel(context.Background())
	stream := &captureLogsStream{
		ctx:   ctx,
		sends: make(chan *v1.LogEntry, 4),
		onSend: func(count int) {
			if count == 1 {
				obs.Logger("test").Info("live")
			}
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- NewService(obs, VisibilityFull).Logs(&v1.LogsRequest{Follow: true}, stream)
	}()

	first := recvLog(t, stream.sends)
	if first.GetMessage() != "replay" {
		t.Fatalf("first log = %+v, want replay", first)
	}
	second := recvLog(t, stream.sends)
	if second.GetMessage() != "live" {
		t.Fatalf("second log = %+v, want live", second)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}
}

type captureEventsStream struct {
	ctx    context.Context
	sends  chan *v1.EventInfo
	onSend func(count int)
	count  int
}

func (s *captureEventsStream) Send(event *v1.EventInfo) error {
	s.count++
	if s.onSend != nil {
		s.onSend(s.count)
	}
	s.sends <- event
	return nil
}

func (s *captureEventsStream) SetHeader(metadata.MD) error  { return nil }
func (s *captureEventsStream) SendHeader(metadata.MD) error { return nil }
func (s *captureEventsStream) SetTrailer(metadata.MD)       {}
func (s *captureEventsStream) Context() context.Context     { return s.ctx }
func (s *captureEventsStream) SendMsg(any) error            { return nil }
func (s *captureEventsStream) RecvMsg(any) error            { return nil }

type captureLogsStream struct {
	ctx    context.Context
	sends  chan *v1.LogEntry
	onSend func(count int)
	count  int
}

func (s *captureLogsStream) Send(entry *v1.LogEntry) error {
	s.count++
	if s.onSend != nil {
		s.onSend(s.count)
	}
	s.sends <- entry
	return nil
}

func (s *captureLogsStream) SetHeader(metadata.MD) error  { return nil }
func (s *captureLogsStream) SendHeader(metadata.MD) error { return nil }
func (s *captureLogsStream) SetTrailer(metadata.MD)       {}
func (s *captureLogsStream) Context() context.Context     { return s.ctx }
func (s *captureLogsStream) SendMsg(any) error            { return nil }
func (s *captureLogsStream) RecvMsg(any) error            { return nil }

func recvEvent(t *testing.T, ch <-chan *v1.EventInfo) *v1.EventInfo {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func recvLog(t *testing.T, ch <-chan *v1.LogEntry) *v1.LogEntry {
	t.Helper()
	select {
	case entry := <-ch:
		return entry
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for log")
		return nil
	}
}
