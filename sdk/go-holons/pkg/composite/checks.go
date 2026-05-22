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

type CheckOutcome struct {
	Pass     bool
	Evidence string
}

type LogCheckOptions struct {
	Conn          *grpc.ClientConn
	Sender        string
	LeafUID       string
	ExpectedChain []string
	Timeout       time.Duration
	PollInterval  time.Duration
	Live          bool
}

type EventCheckOptions struct {
	Conn          *grpc.ClientConn
	EventName     string
	LeafUID       string
	ExpectedChain []string
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

func readLogEntries(ctx context.Context, conn *grpc.ClientConn) ([]observability.LogRecord, error) {
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
		MinSeverityNumber: holonsv1.SeverityNumber_SEVERITY_NUMBER_INFO,
		Follow:            false,
	})
	if err != nil {
		return nil, err
	}
	var out []observability.LogRecord
	for {
		entry, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, observability.FromProtoLogRecord(entry))
	}
}

func readEventEntries(ctx context.Context, conn *grpc.ClientConn) ([]observability.LogRecord, error) {
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
	var out []observability.LogRecord
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, observability.FromProtoLogRecord(event))
	}
}

func matchRelayedLog(entries []observability.LogRecord, opts LogCheckOptions) CheckOutcome {
	for _, entry := range entries {
		if entry.Record.GetBody().GetStringValue() != "tick received" {
			continue
		}
		if observability.StringAttribute(entry.Record.GetAttributes(), "sender") != opts.Sender ||
			observability.StringAttribute(entry.Record.GetAttributes(), "responder_uid") != opts.LeafUID {
			continue
		}
		if evidence := compareChain(entry.Record.GetChain(), opts.ExpectedChain); evidence != "" {
			return CheckOutcome{Evidence: compactCheckEvidence("matching log bad chain: " + evidence)}
		}
		return CheckOutcome{Pass: true}
	}
	return CheckOutcome{Evidence: compactCheckEvidence(fmt.Sprintf("no relayed tick log sender=%s leaf_uid=%s entries=%d", opts.Sender, opts.LeafUID, len(entries)))}
}

func matchRelayedEvent(events []observability.LogRecord, opts EventCheckOptions) CheckOutcome {
	name := opts.EventName
	if name == "" {
		name = observability.EventInstanceReady
	}
	for _, event := range events {
		if event.Record.GetEventName() != name ||
			observability.StringAttribute(event.Record.GetAttributes(), observability.AttrHolonsInstanceUID) != opts.LeafUID {
			continue
		}
		if evidence := compareChain(event.Record.GetChain(), opts.ExpectedChain); evidence != "" {
			return CheckOutcome{Evidence: compactCheckEvidence("matching event bad chain: " + evidence)}
		}
		return CheckOutcome{Pass: true}
	}
	return CheckOutcome{Evidence: compactCheckEvidence(fmt.Sprintf("no relayed %s event leaf_uid=%s events=%d", name, opts.LeafUID, len(events)))}
}

func compareChain(got, want []string) string {
	if len(got) != len(want) {
		return fmt.Sprintf("chain length %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Sprintf("hop %d=%s want %s", i, got[i], want[i])
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
