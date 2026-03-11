// greeting-daemon-go greets users in 56 languages.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	pb "github.com/organic-programming/organic-programming/recipes/daemons/greeting-daemon-go/gen/go/greeting/v1"
	"github.com/organic-programming/organic-programming/recipes/daemons/greeting-daemon-go/internal/server"
	"github.com/organic-programming/go-holons/pkg/grpcclient"
	"github.com/organic-programming/go-holons/pkg/serve"

	"google.golang.org/grpc"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "serve":
		listenURI := serve.ParseFlags(os.Args[2:])
		if err := serve.Run(listenURI, func(gs *grpc.Server) {
			pb.RegisterGreetingServiceServer(gs, &server.Server{})
		}); err != nil {
			fmt.Fprintf(os.Stderr, "serve error: %v\n", err)
			os.Exit(1)
		}
	case "list-languages":
		if err := runListLanguages(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "list-languages error: %v\n", err)
			os.Exit(1)
		}
	case "say-hello":
		if err := runSayHello(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "say-hello error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("greeting-daemon-go v0.4.0")
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: daemon <serve|list-languages|say-hello|version> [flags]")
	fmt.Fprintln(os.Stderr, "  serve   Start the gRPC server (--listen <uri>)")
	fmt.Fprintln(os.Stderr, "  list-languages  Query ListLanguages against a running daemon (--target <uri>)")
	fmt.Fprintln(os.Stderr, "  say-hello       Query SayHello against a running daemon (--target <uri> --lang-code <code> [--name <name>])")
	fmt.Fprintln(os.Stderr, "  version Print version and exit")
	os.Exit(1)
}

func runListLanguages(args []string) error {
	fs := flag.NewFlagSet("list-languages", flag.ContinueOnError)
	target := fs.String("target", "", "daemon target (tcp://host:port or host:port)")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*target) == "" {
		return fmt.Errorf("--target is required")
	}

	client, closeConn, err := newClient(*target)
	if err != nil {
		return err
	}
	defer closeConn()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ListLanguages(ctx, &pb.ListLanguagesRequest{})
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(resp.GetLanguages())
}

func runSayHello(args []string) error {
	fs := flag.NewFlagSet("say-hello", flag.ContinueOnError)
	target := fs.String("target", "", "daemon target (tcp://host:port or host:port)")
	name := fs.String("name", "", "name to greet")
	langCode := fs.String("lang-code", "", "language code")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*target) == "" {
		return fmt.Errorf("--target is required")
	}
	if strings.TrimSpace(*langCode) == "" {
		return fmt.Errorf("--lang-code is required")
	}

	client, closeConn, err := newClient(*target)
	if err != nil {
		return err
	}
	defer closeConn()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SayHello(ctx, &pb.SayHelloRequest{
		Name:     strings.TrimSpace(*name),
		LangCode: strings.TrimSpace(*langCode),
	})
	if err != nil {
		return err
	}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

func newClient(target string) (pb.GreetingServiceClient, func(), error) {
	conn, err := grpcclient.Dial(context.Background(), normalizeTarget(target))
	if err != nil {
		return nil, nil, err
	}
	return pb.NewGreetingServiceClient(conn), func() {
		_ = conn.Close()
	}, nil
}

func normalizeTarget(target string) string {
	trimmed := strings.TrimSpace(target)
	if strings.HasPrefix(trimmed, "tcp://") {
		return strings.TrimPrefix(trimmed, "tcp://")
	}
	return trimmed
}
