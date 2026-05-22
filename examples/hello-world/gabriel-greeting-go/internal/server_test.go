package internal_test

import (
	"context"
	"net"
	"strings"
	"testing"

	pb "gabriel-greeting-go/gen/go/greeting/v1"
	"gabriel-greeting-go/internal"

	v1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/observability"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// startServer launches the GreetingService on an in-memory connection.
func startServer(t *testing.T) pb.GreetingServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterGreetingServiceServer(s, &internal.Server{})

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
	if resp.Greeting != "Bonjour Alice" {
		t.Errorf("expected 'Bonjour Alice', got %q", resp.Greeting)
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
	if resp.Greeting != "Hello Mary" {
		t.Errorf("expected 'Hello Mary', got %q", resp.Greeting)
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
	if resp.Greeting != "Hello Bob" {
		t.Errorf("expected 'Hello Bob', got %q", resp.Greeting)
	}
}

func TestSayHello_EmitsObservabilitySignals(t *testing.T) {
	observability.Reset()
	t.Setenv("OP_OBS", "logs,metrics")
	obs := observability.Configure(observability.Config{Slug: "gabriel-greeting-go", InstanceUID: "greeting-test-uid"})
	t.Cleanup(func() {
		_ = obs.Close()
		observability.Reset()
	})

	resp, err := (&internal.Server{}).SayHello(context.Background(), &pb.SayHelloRequest{
		Name:     " Bob ",
		LangCode: "en",
	})
	if err != nil {
		t.Fatalf("SayHello: %v", err)
	}
	if resp.Greeting != "Hello Bob" {
		t.Fatalf("expected greeting 'Hello Bob', got %q", resp.Greeting)
	}

	snap := observability.Current().Registry().Snapshot()
	var foundCounter bool
	for _, sample := range snap.Counters {
		if sample.Name == "greeting_emitted_total" &&
			sample.Labels["lang_code"] == "en" &&
			sample.Labels["language"] == "English" &&
			sample.Labels["transport"] == "unknown" {
			foundCounter = true
			if sample.Value != 1 {
				t.Fatalf("greeting_emitted_total = %d, want 1", sample.Value)
			}
		}
	}
	if !foundCounter {
		t.Fatalf("missing greeting_emitted_total counter in %+v", snap.Counters)
	}

	var foundLog bool
	for _, entry := range observability.Current().LogRing().Drain() {
		record := observability.ToProtoLogRecord(entry)
		attrs := keyValues(record.GetAttributes())
		body := record.GetBody().GetStringValue()
		if strings.HasPrefix(body, "Greeted ") &&
			strings.HasSuffix(body, " (en)") &&
			attrs["lang_code"].GetStringValue() == "en" {
			foundLog = true
			if body != "Greeted Bob in English (en)" {
				t.Fatalf("unexpected greeting log message: %q", body)
			}
			if record.GetSeverityNumber() != v1.SeverityNumber_SEVERITY_NUMBER_INFO {
				t.Fatalf("severity_number = %v, want INFO", record.GetSeverityNumber())
			}
			if attrs[observability.AttrHolonsSlug].GetStringValue() != "gabriel-greeting-go" ||
				attrs[observability.AttrHolonsInstanceUID].GetStringValue() != "greeting-test-uid" ||
				attrs[observability.AttrServiceName].GetStringValue() != "gabriel-greeting-go" ||
				attrs[observability.AttrServiceInstanceID].GetStringValue() != "greeting-test-uid" {
				t.Fatalf("resource attributes missing or mistyped: %+v", attrs)
			}
			if attrs["language"].GetStringValue() != "English" ||
				attrs["name"].GetStringValue() != "Bob" ||
				attrs["greeting"].GetStringValue() != "Hello Bob" ||
				attrs["transport"].GetStringValue() != "unknown" {
				t.Fatalf("unexpected greeting log attributes: %+v", attrs)
			}
			if _, ok := attrs["duration_ns"].GetValue().(*v1.AnyValue_IntValue); !ok {
				t.Fatalf("duration_ns = %+v, want int_value", attrs["duration_ns"])
			}
			if attrs["duration_ns"].GetIntValue() < 0 {
				t.Fatalf("duration_ns = %d, want non-negative", attrs["duration_ns"].GetIntValue())
			}
		}
	}
	if !foundLog {
		t.Fatal("missing canonical greeting log entry")
	}
}

func keyValues(attrs []*v1.KeyValue) map[string]*v1.AnyValue {
	out := make(map[string]*v1.AnyValue, len(attrs))
	for _, attr := range attrs {
		if attr != nil {
			out[attr.GetKey()] = attr.GetValue()
		}
	}
	return out
}
