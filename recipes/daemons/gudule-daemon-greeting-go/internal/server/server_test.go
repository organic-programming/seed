package server_test

import (
	"context"
	"net"
	"testing"

	pb "github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/gen/go/greeting/v1"
	"github.com/organic-programming/seed/recipes/daemons/gudule-daemon-greeting-go/internal/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// startServer launches the GreetingService on an in-memory connection.
func startServer(t *testing.T) pb.GreetingServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterGreetingServiceServer(s, &server.Server{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("server exited: %v", err)
		}
	}()
	t.Cleanup(s.GracefulStop)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewGreetingServiceClient(conn)
}

func TestListLanguages_ReturnsAll(t *testing.T) {
	client := startServer(t)
	resp, err := client.ListLanguages(context.Background(), &pb.ListLanguagesRequest{})
	if err != nil {
		t.Fatalf("ListLanguages: %v", err)
	}
	if len(resp.Languages) != 56 {
		t.Errorf("expected 56 languages, got %d", len(resp.Languages))
	}
}

func TestListLanguages_HasRequiredFields(t *testing.T) {
	client := startServer(t)
	resp, err := client.ListLanguages(context.Background(), &pb.ListLanguagesRequest{})
	if err != nil {
		t.Fatalf("ListLanguages: %v", err)
	}
	for _, lang := range resp.Languages {
		if lang.Code == "" {
			t.Error("found language with empty Code")
		}
		if lang.Name == "" {
			t.Errorf("language %q has empty Name", lang.Code)
		}
		if lang.Native == "" {
			t.Errorf("language %q has empty Native", lang.Code)
		}
	}
}

func TestSayHello_Nominal(t *testing.T) {
	client := startServer(t)
	resp, err := client.SayHello(context.Background(), &pb.SayHelloRequest{
		Name:     "Alice",
		LangCode: "fr",
	})
	if err != nil {
		t.Fatalf("SayHello: %v", err)
	}
	if resp.Greeting != "Bonjour, Alice !" {
		t.Errorf("expected 'Bonjour, Alice !', got %q", resp.Greeting)
	}
	if resp.Language != "French" {
		t.Errorf("expected language 'French', got %q", resp.Language)
	}
	if resp.LangCode != "fr" {
		t.Errorf("expected lang_code 'fr', got %q", resp.LangCode)
	}
}

func TestSayHello_EmptyName(t *testing.T) {
	client := startServer(t)
	resp, err := client.SayHello(context.Background(), &pb.SayHelloRequest{
		LangCode: "en",
	})
	if err != nil {
		t.Fatalf("SayHello: %v", err)
	}
	if resp.Greeting != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", resp.Greeting)
	}
}

func TestSayHello_UnknownLanguageFallsBackToEnglish(t *testing.T) {
	client := startServer(t)
	resp, err := client.SayHello(context.Background(), &pb.SayHelloRequest{
		Name:     "Bob",
		LangCode: "xx",
	})
	if err != nil {
		t.Fatalf("SayHello: %v", err)
	}
	if resp.LangCode != "en" {
		t.Errorf("expected fallback to 'en', got %q", resp.LangCode)
	}
	if resp.Greeting != "Hello, Bob!" {
		t.Errorf("expected 'Hello, Bob!', got %q", resp.Greeting)
	}
}
