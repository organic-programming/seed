package holonrpc_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

type routingPeer struct {
	label string
	id    string

	client *holonrpc.Client

	onRequest func(map[string]any) (map[string]any, error)

	requestCount      atomic.Int32
	notificationCount atomic.Int32

	requestParamsCh      chan map[string]any
	notificationParamsCh chan map[string]any
}

func newRoutingPeer(label string, onRequest func(map[string]any) (map[string]any, error)) *routingPeer {
	return &routingPeer{
		label:                label,
		onRequest:            onRequest,
		requestParamsCh:      make(chan map[string]any, 64),
		notificationParamsCh: make(chan map[string]any, 64),
	}
}

func connectRoutingPeer(t *testing.T, server *holonrpc.Server, addr string, peer *routingPeer) {
	t.Helper()

	peer.client = holonrpc.NewClient()
	peer.client.Register("peer.v1.Identity/Get", func(context.Context, map[string]any) (map[string]any, error) {
		return map[string]any{"label": peer.label}, nil
	})
	peer.client.Register("Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		cloned := cloneMap(params)
		if isBridgeNotification(cloned) {
			peer.notificationCount.Add(1)
			select {
			case peer.notificationParamsCh <- cloned:
			default:
			}
			return map[string]any{}, nil
		}

		peer.requestCount.Add(1)
		select {
		case peer.requestParamsCh <- cloned:
		default:
		}

		if peer.onRequest != nil {
			return peer.onRequest(cloned)
		}
		return map[string]any{"from": peer.label, "message": cloned["message"]}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := peer.client.Connect(ctx, addr); err != nil {
		t.Fatalf("connect %s: %v", peer.label, err)
	}
	t.Cleanup(func() {
		_ = peer.client.Close()
	})

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel()
	id, err := server.WaitForClient(waitCtx)
	if err != nil {
		t.Fatalf("wait for %s client id: %v", peer.label, err)
	}
	peer.id = id
}

func isBridgeNotification(params map[string]any) bool {
	_, hasPeer := params["peer"]
	_, hasResult := params["result"]
	_, hasError := params["error"]
	return hasPeer && (hasResult || hasError)
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func recvParams(t *testing.T, ch <-chan map[string]any, timeout time.Duration, label string) map[string]any {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(timeout):
		t.Fatalf("timeout waiting for %s params", label)
		return nil
	}
}

func parseFanOutEntries(t *testing.T, result map[string]any) []map[string]any {
	t.Helper()
	raw, ok := result["value"]
	if !ok {
		t.Fatalf("missing wrapped fan-out array in result: %#v", result)
	}
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("fan-out result value type = %T, want []any", raw)
	}
	entries := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("fan-out entry type = %T, want map[string]any", item)
		}
		entries = append(entries, entry)
	}
	return entries
}

func assertRoutingFieldStripped(t *testing.T, params map[string]any) {
	t.Helper()
	if _, ok := params["_routing"]; ok {
		t.Fatalf("handler received _routing field: %#v", params)
	}
	if _, ok := params["_peer"]; ok {
		t.Fatalf("handler received _peer field: %#v", params)
	}
}

func waitForCount(t *testing.T, counter *atomic.Int32, want int32, timeout time.Duration, label string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if counter.Load() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s count = %d, want %d", label, counter.Load(), want)
}

func TestHolonRPC_Routing_Unicast_TargetPeerID(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	peerB := newRoutingPeer("B", nil)
	peerC := newRoutingPeer("C", nil)
	peerD := newRoutingPeer("D", nil)

	connectRoutingPeer(t, server, addr, peerA)
	connectRoutingPeer(t, server, addr, peerB)
	connectRoutingPeer(t, server, addr, peerC)
	connectRoutingPeer(t, server, addr, peerD)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := peerA.client.Invoke(ctx, "Echo/Ping", map[string]any{
		"_peer":   peerB.id,
		"message": "hello-unicast",
	})
	if err != nil {
		t.Fatalf("unicast invoke: %v", err)
	}
	if got, _ := resp["from"].(string); got != "B" {
		t.Fatalf("unicast response from = %q, want B", got)
	}

	waitForCount(t, &peerB.requestCount, 1, 2*time.Second, "peer B request")
	if peerC.requestCount.Load() != 0 || peerD.requestCount.Load() != 0 {
		t.Fatalf("unexpected non-target requests: C=%d D=%d", peerC.requestCount.Load(), peerD.requestCount.Load())
	}

	params := recvParams(t, peerB.requestParamsCh, 2*time.Second, "peer B request")
	assertRoutingFieldStripped(t, params)
}

func TestHolonRPC_Routing_FanOut_AggregatesResults(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	peerB := newRoutingPeer("B", nil)
	peerC := newRoutingPeer("C", nil)
	peerD := newRoutingPeer("D", nil)

	connectRoutingPeer(t, server, addr, peerA)
	connectRoutingPeer(t, server, addr, peerB)
	connectRoutingPeer(t, server, addr, peerC)
	connectRoutingPeer(t, server, addr, peerD)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := peerA.client.Invoke(ctx, "*.Echo/Ping", map[string]any{"message": "hello-fanout"})
	if err != nil {
		t.Fatalf("fan-out invoke: %v", err)
	}

	entries := parseFanOutEntries(t, resp)
	if len(entries) != 3 {
		t.Fatalf("fan-out result count = %d, want 3", len(entries))
	}

	seenPeers := map[string]bool{}
	for _, entry := range entries {
		peerID, _ := entry["peer"].(string)
		if peerID == "" {
			t.Fatalf("fan-out entry missing peer id: %#v", entry)
		}
		seenPeers[peerID] = true
		if _, ok := entry["result"].(map[string]any); !ok {
			t.Fatalf("fan-out entry missing result object: %#v", entry)
		}
	}
	for _, wantPeer := range []string{peerB.id, peerC.id, peerD.id} {
		if !seenPeers[wantPeer] {
			t.Fatalf("missing fan-out entry for peer %s", wantPeer)
		}
	}
}

func TestHolonRPC_Routing_FanOut_PartialFailure(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	peerB := newRoutingPeer("B", nil)
	peerC := newRoutingPeer("C", func(map[string]any) (map[string]any, error) {
		return nil, &holonrpc.ResponseError{Code: 4, Message: "deadline exceeded"}
	})
	peerD := newRoutingPeer("D", nil)

	connectRoutingPeer(t, server, addr, peerA)
	connectRoutingPeer(t, server, addr, peerB)
	connectRoutingPeer(t, server, addr, peerC)
	connectRoutingPeer(t, server, addr, peerD)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := peerA.client.Invoke(ctx, "*.Echo/Ping", map[string]any{"message": "hello-partial"})
	if err != nil {
		t.Fatalf("fan-out invoke: %v", err)
	}

	entries := parseFanOutEntries(t, resp)
	if len(entries) != 3 {
		t.Fatalf("fan-out result count = %d, want 3", len(entries))
	}

	errorCount := 0
	successCount := 0
	for _, entry := range entries {
		if errObj, ok := entry["error"].(map[string]any); ok {
			errorCount++
			if code, _ := errObj["code"].(float64); int(code) != 4 {
				t.Fatalf("error code = %v, want 4", code)
			}
			continue
		}
		if _, ok := entry["result"].(map[string]any); ok {
			successCount++
		}
	}

	if errorCount != 1 || successCount != 2 {
		t.Fatalf("partial failure counts: success=%d error=%d, want 2 and 1", successCount, errorCount)
	}
}

func TestHolonRPC_Routing_FanOut_EmptyReturnsNotFound(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	connectRoutingPeer(t, server, addr, peerA)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := peerA.client.Invoke(ctx, "*.Echo/Ping", map[string]any{"message": "nobody"})
	requireRPCErrorCode(t, err, 5)
}

func TestHolonRPC_Routing_BroadcastResponse(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	peerB := newRoutingPeer("B", nil)
	peerC := newRoutingPeer("C", nil)
	peerD := newRoutingPeer("D", nil)

	connectRoutingPeer(t, server, addr, peerA)
	connectRoutingPeer(t, server, addr, peerB)
	connectRoutingPeer(t, server, addr, peerC)
	connectRoutingPeer(t, server, addr, peerD)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := peerA.client.Invoke(ctx, "Echo/Ping", map[string]any{
		"_peer":    peerB.id,
		"_routing": "broadcast-response",
		"message":  "hello-broadcast-response",
	})
	if err != nil {
		t.Fatalf("broadcast-response invoke: %v", err)
	}
	if got, _ := resp["from"].(string); got != "B" {
		t.Fatalf("broadcast-response caller result from = %q, want B", got)
	}

	bParams := recvParams(t, peerB.requestParamsCh, 2*time.Second, "peer B request")
	assertRoutingFieldStripped(t, bParams)

	notifC := recvParams(t, peerC.notificationParamsCh, 2*time.Second, "peer C notification")
	notifD := recvParams(t, peerD.notificationParamsCh, 2*time.Second, "peer D notification")
	for _, notif := range []map[string]any{notifC, notifD} {
		peerID, _ := notif["peer"].(string)
		if peerID != peerB.id {
			t.Fatalf("notification peer = %q, want %q", peerID, peerB.id)
		}
		if _, ok := notif["result"].(map[string]any); !ok {
			t.Fatalf("notification missing result map: %#v", notif)
		}
	}

	if peerB.notificationCount.Load() != 0 {
		t.Fatalf("target peer B should not receive broadcast notification, got %d", peerB.notificationCount.Load())
	}
}

func TestHolonRPC_Routing_FullBroadcast(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	peerB := newRoutingPeer("B", nil)
	peerC := newRoutingPeer("C", nil)
	peerD := newRoutingPeer("D", nil)

	connectRoutingPeer(t, server, addr, peerA)
	connectRoutingPeer(t, server, addr, peerB)
	connectRoutingPeer(t, server, addr, peerC)
	connectRoutingPeer(t, server, addr, peerD)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := peerA.client.Invoke(ctx, "*.Echo/Ping", map[string]any{
		"message":  "hello-full-broadcast",
		"_routing": "full-broadcast",
	})
	if err != nil {
		t.Fatalf("full-broadcast invoke: %v", err)
	}

	entries := parseFanOutEntries(t, resp)
	if len(entries) != 3 {
		t.Fatalf("full-broadcast aggregate count = %d, want 3", len(entries))
	}

	for _, p := range []*routingPeer{peerB, peerC, peerD} {
		params := recvParams(t, p.requestParamsCh, 2*time.Second, fmt.Sprintf("peer %s request", p.label))
		assertRoutingFieldStripped(t, params)
	}

	waitForCount(t, &peerB.notificationCount, 2, 3*time.Second, "peer B notifications")
	waitForCount(t, &peerC.notificationCount, 2, 3*time.Second, "peer C notifications")
	waitForCount(t, &peerD.notificationCount, 2, 3*time.Second, "peer D notifications")

	for _, p := range []*routingPeer{peerB, peerC, peerD} {
		seenFrom := map[string]bool{}
		for i := 0; i < 2; i++ {
			notif := recvParams(t, p.notificationParamsCh, 2*time.Second, fmt.Sprintf("peer %s notification", p.label))
			fromPeerID, _ := notif["peer"].(string)
			if fromPeerID == p.id {
				t.Fatalf("peer %s received self-response notification: %#v", p.label, notif)
			}
			seenFrom[fromPeerID] = true
			if _, ok := notif["result"].(map[string]any); !ok {
				t.Fatalf("peer %s notification missing result map: %#v", p.label, notif)
			}
		}
		if len(seenFrom) != 2 {
			t.Fatalf("peer %s notification distinct sources = %d, want 2", p.label, len(seenFrom))
		}
	}
}

func TestHolonRPC_Routing_ConcurrentFanOut(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	peerB := newRoutingPeer("B", nil)
	peerC := newRoutingPeer("C", nil)
	peerD := newRoutingPeer("D", nil)

	connectRoutingPeer(t, server, addr, peerA)
	connectRoutingPeer(t, server, addr, peerB)
	connectRoutingPeer(t, server, addr, peerC)
	connectRoutingPeer(t, server, addr, peerD)

	const goroutines = 10
	errCh := make(chan error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()

			resp, err := peerA.client.Invoke(ctx, "*.Echo/Ping", map[string]any{"message": fmt.Sprintf("m-%d", i)})
			if err != nil {
				errCh <- err
				return
			}

			entries := parseFanOutEntries(t, resp)
			if len(entries) != 3 {
				errCh <- fmt.Errorf("fan-out entries = %d, want 3", len(entries))
				return
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHolonRPC_Routing_Unicast_UnknownPeerReturnsNotFound(t *testing.T) {
	server, addr := startHolonRPCServer(t, nil)

	peerA := newRoutingPeer("A", nil)
	connectRoutingPeer(t, server, addr, peerA)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := peerA.client.Invoke(ctx, "Echo/Ping", map[string]any{
		"_peer":   "missing-peer-id",
		"message": "hello",
	})
	requireRPCErrorCode(t, err, 5)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown peer error message = %v, want contains not found", err)
	}
}
