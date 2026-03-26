package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/organic-programming/go-holons/pkg/describe"
	"google.golang.org/protobuf/encoding/protojson"
)

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "describe-json-go: %v\n", err)
		os.Exit(2)
	}

	response, err := describe.BuildResponse(cfg.protoDir, cfg.manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "describe-json-go: build response: %v\n", err)
		os.Exit(1)
	}

	data, err := protojson.MarshalOptions{
		Indent:          "  ",
		UseProtoNames:   true,
		EmitUnpopulated: true,
	}.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "describe-json-go: encode response: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "describe-json-go: create output dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(cfg.out, append(data, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "describe-json-go: write %s: %v\n", cfg.out, err)
		os.Exit(1)
	}
}

type config struct {
	protoDir string
	manifest string
	out      string
}

func parseArgs(args []string) (config, error) {
	fs := flag.NewFlagSet("describe-json-go", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	cfg := config{}
	fs.StringVar(&cfg.protoDir, "proto-dir", "", "proto directory used to build the DescribeResponse")
	fs.StringVar(&cfg.manifest, "manifest", "", "manifest path for the holon")
	fs.StringVar(&cfg.out, "out", "", "output JSON path")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.protoDir) == "" {
		return cfg, fmt.Errorf("--proto-dir is required")
	}
	if strings.TrimSpace(cfg.manifest) == "" {
		return cfg, fmt.Errorf("--manifest is required")
	}
	if strings.TrimSpace(cfg.out) == "" {
		return cfg, fmt.Errorf("--out is required")
	}

	return cfg, nil
}
