package describe_test

import (
	"context"
	"errors"
	"testing"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/transport"

	"google.golang.org/grpc"
)

func TestRegisterRequiresStaticDescribeResponse(t *testing.T) {
	describe.UseStaticResponse(nil)
	t.Cleanup(func() { describe.UseStaticResponse(nil) })

	server := grpc.NewServer()
	if err := describe.Register(server); !errors.Is(err, describe.ErrNoIncodeDescription) {
		t.Fatalf("Register error = %v, want %v", err, describe.ErrNoIncodeDescription)
	}
}

func TestRegisterServesStaticDescribeResponse(t *testing.T) {
	describe.UseStaticResponse(sampleStaticDescribeResponse())
	t.Cleanup(func() { describe.UseStaticResponse(nil) })

	listener, err := transport.Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	server := grpc.NewServer()
	if err := describe.Register(server); err != nil {
		t.Fatalf("Register: %v", err)
	}

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = <-serveErrCh
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpcclient.Dial(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	response, err := holonsv1.NewHolonMetaClient(conn).Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if got := describeResponseSlug(response); got != "static-holon" {
		t.Fatalf("slug = %q, want %q", got, "static-holon")
	}
	if len(response.GetServices()) != 1 {
		t.Fatalf("services len = %d, want 1", len(response.GetServices()))
	}
	if got := response.GetServices()[0].GetName(); got != "static.v1.Echo" {
		t.Fatalf("service name = %q, want %q", got, "static.v1.Echo")
	}
}

func sampleStaticDescribeResponse() *holonsv1.DescribeResponse {
	return &holonsv1.DescribeResponse{
		Manifest: &holonsv1.HolonManifest{
			Identity: &holonsv1.HolonManifest_Identity{
				Schema:     "holon/v1",
				Uuid:       "static-holon-0000",
				GivenName:  "Static",
				FamilyName: "Holon",
				Motto:      "Registered at runtime from generated code.",
				Composer:   "describe-test",
				Status:     "draft",
				Born:       "2026-03-23",
			},
			Lang: "go",
		},
		Services: []*holonsv1.ServiceDoc{{
			Name:        "static.v1.Echo",
			Description: "Static test service.",
			Methods: []*holonsv1.MethodDoc{{
				Name:        "Ping",
				Description: "Replies with the payload.",
			}},
		}},
	}
}
