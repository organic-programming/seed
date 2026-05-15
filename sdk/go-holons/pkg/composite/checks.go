package composite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/observability"
	"google.golang.org/grpc"
)

type ChainHop = observability.Hop

type CheckOutcome struct {
	Pass     bool
	Evidence string
}

type LogCheckOptions struct {
	Conn          *grpc.ClientConn
	Sender        string
	LeafUID       string
	ExpectedChain []ChainHop
	Timeout       time.Duration
	PollInterval  time.Duration
	Live          bool
}

type EventCheckOptions struct {
	Conn          *grpc.ClientConn
	EventType     observability.EventType
	LeafUID       string
	ExpectedChain []ChainHop
	Timeout       time.Duration
	PollInterval  time.Duration
	Live          bool
}

func CheckRelayedLog(ctx context.Context, opts LogCheckOptions) CheckOutcome {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	interval := opts.PollInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	var last CheckOutcome
	for {
		entries, err := readLogEntries(ctx, opts.Conn)
		if err != nil {
			last = CheckOutcome{Evidence: compactCheckEvidence(err.Error())}
		} else {
			last = matchRelayedLog(entries, opts)
			if last.Pass {
				return last
			}
		}
		if time.Now().After(deadline) {
			return last
		}
		select {
		case <-ctx.Done():
			if last.Evidence == "" {
				last.Evidence = ctx.Err().Error()
			}
			return last
		case <-time.After(interval):
		}
	}
}

func CheckRelayedEvent(ctx context.Context, opts EventCheckOptions) CheckOutcome {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	interval := opts.PollInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	deadline := time.Now().Add(timeout)
	var last CheckOutcome
	for {
		events, err := readEventEntries(ctx, opts.Conn)
		if err != nil {
			last = CheckOutcome{Evidence: compactCheckEvidence(err.Error())}
		} else {
			last = matchRelayedEvent(events, opts)
			if last.Pass {
				return last
			}
		}
		if time.Now().After(deadline) {
			return last
		}
		select {
		case <-ctx.Done():
			if last.Evidence == "" {
				last.Evidence = ctx.Err().Error()
			}
			return last
		case <-time.After(interval):
		}
	}
}

func readLogEntries(ctx context.Context, conn *grpc.ClientConn) ([]observability.LogEntry, error) {
	if conn == nil {
		ring := observability.Current().LogRing()
		if ring == nil {
			return nil, fmt.Errorf("logs family is not enabled")
		}
		return ring.Drain(), nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	stream, err := holonsv1.NewHolonObservabilityClient(conn).Logs(reqCtx, &holonsv1.LogsRequest{
		MinLevel: holonsv1.LogLevel_INFO,
		Follow:   false,
	})
	if err != nil {
		return nil, err
	}
	var out []observability.LogEntry
	for {
		entry, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, observability.FromProtoLogEntry(entry))
	}
}

func readEventEntries(ctx context.Context, conn *grpc.ClientConn) ([]observability.Event, error) {
	if conn == nil {
		bus := observability.Current().EventBus()
		if bus == nil {
			return nil, fmt.Errorf("events family is not enabled")
		}
		return bus.Drain(), nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	stream, err := holonsv1.NewHolonObservabilityClient(conn).Events(reqCtx, &holonsv1.EventsRequest{Follow: false})
	if err != nil {
		return nil, err
	}
	var out []observability.Event
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, observability.FromProtoEvent(event))
	}
}

func matchRelayedLog(entries []observability.LogEntry, opts LogCheckOptions) CheckOutcome {
	for _, entry := range entries {
		if entry.Message != "tick received" {
			continue
		}
		if entry.Fields["sender"] != opts.Sender || entry.Fields["responder_uid"] != opts.LeafUID {
			continue
		}
		if evidence := compareChain(entry.Chain, opts.ExpectedChain); evidence != "" {
			return CheckOutcome{Evidence: compactCheckEvidence("matching log bad chain: " + evidence)}
		}
		return CheckOutcome{Pass: true}
	}
	return CheckOutcome{Evidence: compactCheckEvidence(fmt.Sprintf("no relayed tick log sender=%s leaf_uid=%s entries=%d", opts.Sender, opts.LeafUID, len(entries)))}
}

func matchRelayedEvent(events []observability.Event, opts EventCheckOptions) CheckOutcome {
	typ := opts.EventType
	if typ == observability.EventTypeUnspecified {
		typ = observability.EventInstanceReady
	}
	for _, event := range events {
		if event.Type != typ || event.InstanceUID != opts.LeafUID {
			continue
		}
		if evidence := compareChain(event.Chain, opts.ExpectedChain); evidence != "" {
			return CheckOutcome{Evidence: compactCheckEvidence("matching event bad chain: " + evidence)}
		}
		return CheckOutcome{Pass: true}
	}
	return CheckOutcome{Evidence: compactCheckEvidence(fmt.Sprintf("no relayed %s event leaf_uid=%s events=%d", typ.String(), opts.LeafUID, len(events)))}
}

func compareChain(got, want []ChainHop) string {
	if len(got) != len(want) {
		return fmt.Sprintf("chain length %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Slug != want[i].Slug || got[i].InstanceUID != want[i].InstanceUID {
			return fmt.Sprintf("hop %d=%s/%s want %s/%s", i, got[i].Slug, got[i].InstanceUID, want[i].Slug, want[i].InstanceUID)
		}
	}
	return ""
}

func compactCheckEvidence(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 240 {
		return value
	}
	return value[:240] + "..."
}
