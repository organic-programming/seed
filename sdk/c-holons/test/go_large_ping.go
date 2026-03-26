package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const dialTimeout = 8 * time.Second

type PingRequest struct {
	Message string `json:"message"`
}

type PingResponse struct {
	Message string `json:"message"`
	SDK     string `json:"sdk"`
	Version string `json:"version"`
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: go_large_ping.go <tcp://host:port|unix://path>\n")
		os.Exit(2)
	}

	target, dialer, err := normalizeTarget(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid target: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
	defer cancel()

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(jsonCodec{})),
	}
	if dialer != nil {
		dialOpts = append(dialOpts, grpc.WithContextDialer(dialer))
	}

	//nolint:staticcheck // DialContext is required for custom dialers + blocking connect.
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = conn.Close()
	}()

	var largeResp PingResponse
	largeErr := invokePing(ctx, conn, strings.Repeat("a", 2*1024*1024), &largeResp)
	if status.Code(largeErr) != codes.ResourceExhausted {
		if largeErr == nil {
			fmt.Fprintf(os.Stderr, "expected RESOURCE_EXHAUSTED, got success len=%d\n", len(largeResp.Message))
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "expected RESOURCE_EXHAUSTED, got %v (%v)\n", status.Code(largeErr), largeErr)
		os.Exit(1)
	}

	var smallResp PingResponse
	if err := invokePing(ctx, conn, "small", &smallResp); err != nil {
		fmt.Fprintf(os.Stderr, "follow-up ping failed: %v\n", err)
		os.Exit(1)
	}
	if smallResp.Message != "small" {
		fmt.Fprintf(os.Stderr, "follow-up message mismatch: got=%q want=%q\n", smallResp.Message, "small")
		os.Exit(1)
	}

	fmt.Printf("RESULT=RESOURCE_EXHAUSTED SMALL_OK SDK=%s\n", smallResp.SDK)
}

func invokePing(ctx context.Context, conn *grpc.ClientConn, message string, out *PingResponse) error {
	return conn.Invoke(
		ctx,
		"/echo.v1.Echo/Ping",
		&PingRequest{Message: message},
		out,
		grpc.ForceCodec(jsonCodec{}),
	)
}

func normalizeTarget(uri string) (string, func(context.Context, string) (net.Conn, error), error) {
	if !strings.Contains(uri, "://") {
		return uri, nil, nil
	}

	if strings.HasPrefix(uri, "tcp://") {
		return strings.TrimPrefix(uri, "tcp://"), nil, nil
	}

	if strings.HasPrefix(uri, "unix://") {
		path := strings.TrimPrefix(uri, "unix://")
		dialer := func(_ context.Context, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", path, 5*time.Second)
		}
		return "passthrough:///unix", dialer, nil
	}

	return "", nil, fmt.Errorf("unsupported URI: %s", uri)
}
