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
		sends: make(chan *v1.LogRecord, 4),
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
	if first.GetEventName() != EventInstanceReady || StringAttribute(first.GetAttributes(), "phase") != "replay" {
		t.Fatalf("first event = %+v, want replay instance.ready", first)
	}
	second := recvEvent(t, stream.sends)
	if second.GetEventName() != EventInstanceExited || StringAttribute(second.GetAttributes(), "phase") != "live" {
		t.Fatalf("second event = %+v, want live instance.exited", second)
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
		sends: make(chan *v1.LogRecord, 4),
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
	if first.GetBody().GetStringValue() != "replay" {
		t.Fatalf("first log = %+v, want replay", first)
	}
	second := recvLog(t, stream.sends)
	if second.GetBody().GetStringValue() != "live" {
		t.Fatalf("second log = %+v, want live", second)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Logs returned error: %v", err)
	}
}

type captureEventsStream struct {
	ctx    context.Context
	sends  chan *v1.LogRecord
	onSend func(count int)
	count  int
}

func (s *captureEventsStream) Send(event *v1.LogRecord) error {
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
	sends  chan *v1.LogRecord
	onSend func(count int)
	count  int
}

func (s *captureLogsStream) Send(entry *v1.LogRecord) error {
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

func recvEvent(t *testing.T, ch <-chan *v1.LogRecord) *v1.LogRecord {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
		return nil
	}
}

func recvLog(t *testing.T, ch <-chan *v1.LogRecord) *v1.LogRecord {
	t.Helper()
	select {
	case entry := <-ch:
		return entry
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for log")
		return nil
	}
}
