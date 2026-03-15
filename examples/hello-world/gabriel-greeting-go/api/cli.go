package api

import (
	"fmt"
	"io"
	"os"
	"strings"

	pb "gabriel-greeting-go/gen/go/greeting/v1"
	"gabriel-greeting-go/internal"

	"github.com/organic-programming/go-holons/pkg/serve"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const Version = "gudule-daemon-greeting-go v0.4.1"

type outputFormat string

const (
	formatText outputFormat = "text"
	formatJSON outputFormat = "json"
)

type commandOptions struct {
	format outputFormat
	lang   string
}

// RunCLI dispatches the Gabriel CLI and returns a process exit code.
// Optional writers keep tests simple while callers can just pass args.
func RunCLI(args []string, outputs ...io.Writer) int {
	stdout := io.Writer(os.Stdout)
	stderr := io.Writer(os.Stderr)
	if len(outputs) > 0 && outputs[0] != nil {
		stdout = outputs[0]
	}
	if len(outputs) > 1 && outputs[1] != nil {
		stderr = outputs[1]
	}

	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch canonicalCommand(args[0]) {
	case "serve":
		listenURI := serve.ParseFlags(args[1:])
		if err := internal.ListenAndServe(listenURI, true); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		return 0
	case "version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "help":
		printUsage(stdout)
		return 0
	case "listlanguages":
		return runListLanguages(args[1:], stdout, stderr)
	case "sayhello":
		return runSayHello(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func runListLanguages(args []string, stdout, stderr io.Writer) int {
	opts, positional, err := parseCommandOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "listLanguages: %v\n", err)
		return 1
	}
	if len(positional) != 0 {
		fmt.Fprintln(stderr, "listLanguages: accepts no positional arguments")
		return 1
	}

	response, err := ListLanguages()
	if err != nil {
		fmt.Fprintf(stderr, "listLanguages: %v\n", err)
		return 1
	}
	if err := writeProto(stdout, response, opts.format); err != nil {
		fmt.Fprintf(stderr, "listLanguages: %v\n", err)
		return 1
	}
	return 0
}

func runSayHello(args []string, stdout, stderr io.Writer) int {
	opts, positional, err := parseCommandOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "sayHello: %v\n", err)
		return 1
	}
	if len(positional) > 2 {
		fmt.Fprintln(stderr, "sayHello: accepts at most <name> [lang_code]")
		return 1
	}

	req := &pb.SayHelloRequest{LangCode: "en"}
	if len(positional) >= 1 {
		req.Name = positional[0]
	}
	if len(positional) >= 2 {
		if opts.lang != "" {
			fmt.Fprintln(stderr, "sayHello: use either a positional lang_code or --lang, not both")
			return 1
		}
		req.LangCode = positional[1]
	}
	if opts.lang != "" {
		req.LangCode = opts.lang
	}

	response, err := SayHello(req)
	if err != nil {
		fmt.Fprintf(stderr, "sayHello: %v\n", err)
		return 1
	}
	if err := writeProto(stdout, response, opts.format); err != nil {
		fmt.Fprintf(stderr, "sayHello: %v\n", err)
		return 1
	}
	return 0
}

func parseCommandOptions(args []string) (commandOptions, []string, error) {
	opts := commandOptions{format: formatText}
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--json":
			opts.format = formatJSON
		case arg == "--format":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--format requires a value")
			}
			format, err := parseOutputFormat(args[i+1])
			if err != nil {
				return opts, nil, err
			}
			opts.format = format
			i++
		case strings.HasPrefix(arg, "--format="):
			format, err := parseOutputFormat(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				return opts, nil, err
			}
			opts.format = format
		case arg == "--lang":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--lang requires a value")
			}
			opts.lang = strings.TrimSpace(args[i+1])
			i++
		case strings.HasPrefix(arg, "--lang="):
			opts.lang = strings.TrimSpace(strings.TrimPrefix(arg, "--lang="))
		case strings.HasPrefix(arg, "--"):
			return opts, nil, fmt.Errorf("unknown flag %q", arg)
		default:
			positional = append(positional, arg)
		}
	}

	return opts, positional, nil
}

func parseOutputFormat(raw string) (outputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "text", "txt":
		return formatText, nil
	case "json":
		return formatJSON, nil
	default:
		return "", fmt.Errorf("unsupported format %q", raw)
	}
}

func writeProto(w io.Writer, msg proto.Message, format outputFormat) error {
	switch format {
	case formatJSON:
		data, err := protojson.MarshalOptions{Indent: "  "}.Marshal(msg)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s\n", data)
		return err
	case formatText:
		return writeText(w, msg)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeText(w io.Writer, msg proto.Message) error {
	switch typed := msg.(type) {
	case *pb.SayHelloResponse:
		_, err := fmt.Fprintln(w, typed.GetGreeting())
		return err
	case *pb.ListLanguagesResponse:
		for _, lang := range typed.GetLanguages() {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", lang.GetCode(), lang.GetName(), lang.GetNative()); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported text output for %T", msg)
	}
}

func canonicalCommand(raw string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(strings.ToLower(strings.TrimSpace(raw)))
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: gabriel-greeting-go <command> [args] [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  serve [--listen <uri>]                    Start the gRPC server")
	fmt.Fprintln(w, "  version                                  Print version and exit")
	fmt.Fprintln(w, "  listLanguages [--format text|json]       List supported languages")
	fmt.Fprintln(w, "  sayHello [name] [lang_code] [--format text|json] [--lang <code>]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "examples:")
	fmt.Fprintln(w, "  gabriel-greeting-go serve --listen stdio")
	fmt.Fprintln(w, "  gabriel-greeting-go listLanguages --format json")
	fmt.Fprintln(w, "  gabriel-greeting-go sayHello Alice fr")
	fmt.Fprintln(w, "  gabriel-greeting-go sayHello Alice --lang fr --format json")
}
