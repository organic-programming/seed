package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	grpcclientpkg "github.com/organic-programming/grace-op/internal/grpcclient"
	toolspkg "github.com/organic-programming/grace-op/internal/tools"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewServerFromURI connects to an already-running gRPC server via URI and
// prepares MCP tool/prompt state. It tries HolonMeta/Describe first and
// falls back to gRPC reflection for servers that don't implement Describe.
func NewServerFromURI(uri string, version string) (*Server, error) {
	address, err := parseGRPCAddress(uri)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", uri, err)
	}

	// Try Describe with a very short timeout — remote COAX servers typically
	// don't implement HolonMeta, and some never return Unimplemented either,
	// so we need a fast cutoff before falling back to reflection.
	describeCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	response, describeErr := holonsv1.NewHolonMetaClient(conn).Describe(describeCtx, &holonsv1.DescribeRequest{})
	cancel()

	if describeErr == nil && response != nil {
		return buildServerFromDescribe(conn, response, uri, version)
	}

	// Describe unavailable — fall back to reflection-based discovery.
	return buildServerFromReflection(conn, address, uri, version)
}

// buildServerFromDescribe builds the MCP server using a DescribeResponse
// (same pipeline as NewServer for local holons).
func buildServerFromDescribe(conn *grpc.ClientConn, response *holonsv1.DescribeResponse, uri string, version string) (*Server, error) {
	slug := slugFromDescribe(response, uri)

	connCache := map[string]*grpc.ClientConn{slug: conn}
	describeCache := map[string]*holonsv1.DescribeResponse{slug: response}
	toolNamesBySlug := map[string][]string{}

	definitions := toolspkg.DefinitionsFromDescribe(slug, response)
	definitionIndex := make(map[string]toolspkg.Definition, len(definitions))
	for _, definition := range definitions {
		definitionIndex[definition.Name] = definition
		toolNamesBySlug[slug] = append(toolNamesBySlug[slug], definition.Name)
	}

	toolIndex := make(map[string]toolBinding, len(definitions))
	for _, service := range response.GetServices() {
		serviceName := shortName(service.GetName())
		if serviceName == "" {
			continue
		}
		for _, method := range service.GetMethods() {
			methodName := strings.TrimSpace(method.GetName())
			if methodName == "" {
				continue
			}

			name := slug + "." + serviceName + "." + methodName
			definition, ok := definitionIndex[name]
			if !ok {
				continue
			}

			toolIndex[name] = toolBinding{
				slug:            slug,
				definition:      definition,
				fullMethod:      "/" + strings.TrimSpace(service.GetName()) + "/" + methodName,
				clientStreaming: method.GetClientStreaming(),
				serverStreaming: method.GetServerStreaming(),
			}
		}
	}

	for _, sequence := range response.GetManifest().GetSequences() {
		sequenceName := strings.TrimSpace(sequence.GetName())
		if sequenceName == "" {
			continue
		}
		name := slug + ".sequence." + sequenceName
		definition, ok := definitionIndex[name]
		if !ok {
			continue
		}
		toolIndex[name] = toolBinding{
			slug:       slug,
			definition: definition,
			sequence:   sequence,
		}
	}

	prompts := buildPromptDefinitions([]string{slug}, describeCache, toolNamesBySlug)
	promptIndex := make(map[string]promptDefinition, len(prompts))
	for _, prompt := range prompts {
		promptIndex[prompt.Name] = prompt
	}

	return &Server{
		version:       version,
		tools:         definitions,
		prompts:       prompts,
		toolIndex:     toolIndex,
		promptIndex:   promptIndex,
		connCache:     connCache,
		describeCache: describeCache,
	}, nil
}

// buildServerFromReflection builds the MCP server using gRPC reflection
// when Describe is unavailable (typical for COAX composite servers).
// Each reflected method becomes a tool with a minimal schema (accepts any JSON).
func buildServerFromReflection(conn *grpc.ClientConn, address string, uri string, version string) (*Server, error) {
	methods, err := grpcclientpkg.ListMethods(address)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("list methods at %s: %w", uri, err)
	}

	slug := slugFromURI(uri)
	connCache := map[string]*grpc.ClientConn{slug: conn}

	definitions := make([]toolspkg.Definition, 0, len(methods))
	toolIndex := make(map[string]toolBinding, len(methods))

	for _, fullMethod := range methods {
		// fullMethod is "package.Service/Method"
		parts := strings.SplitN(fullMethod, "/", 2)
		if len(parts) != 2 {
			continue
		}
		serviceFQN := parts[0]
		methodName := parts[1]
		serviceName := shortName(serviceFQN)

		toolName := slug + "." + serviceName + "." + methodName
		definition := toolspkg.Definition{
			Name:        toolName,
			Description: fmt.Sprintf("%s.%s (via reflection)", serviceName, methodName),
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		}

		definitions = append(definitions, definition)
		toolIndex[toolName] = toolBinding{
			slug:       slug,
			definition: definition,
			fullMethod: "/" + fullMethod,
		}
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})

	return &Server{
		version:       version,
		tools:         definitions,
		prompts:       nil,
		toolIndex:     toolIndex,
		promptIndex:   map[string]promptDefinition{},
		connCache:     connCache,
		describeCache: map[string]*holonsv1.DescribeResponse{},
	}, nil
}

// IsURI returns true if the argument looks like a gRPC URI (contains "://").
func IsURI(arg string) bool {
	return strings.Contains(arg, "://")
}

// parseGRPCAddress extracts a host:port from a URI like "grpc+tcp://host:port".
func parseGRPCAddress(uri string) (string, error) {
	trimmed := strings.TrimSpace(uri)
	if trimmed == "" {
		return "", fmt.Errorf("empty URI")
	}

	// Strip known scheme prefixes.
	for _, prefix := range []string{"grpc+tcp://", "tcp://", "grpc://"} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimPrefix(trimmed, prefix), nil
		}
	}

	// If no known scheme, check if it has a scheme at all.
	if idx := strings.Index(trimmed, "://"); idx >= 0 {
		return "", fmt.Errorf("unsupported URI scheme in %q (use grpc+tcp://, tcp://, or grpc://)", trimmed)
	}

	// Bare host:port.
	return trimmed, nil
}

// slugFromDescribe derives a slug from the DescribeResponse identity.
func slugFromDescribe(response *holonsv1.DescribeResponse, fallback string) string {
	if identity := response.GetManifest().GetIdentity(); identity != nil {
		given := strings.TrimSpace(identity.GetGivenName())
		family := strings.TrimSpace(identity.GetFamilyName())
		if given != "" && family != "" {
			return given + "-" + family
		}
		if given != "" {
			return given
		}
	}
	return slugFromURI(fallback)
}

// slugFromURI derives a slug from the URI (host part or full URI as fallback).
func slugFromURI(uri string) string {
	address, _ := parseGRPCAddress(uri)
	if host, _, ok := strings.Cut(address, ":"); ok && host != "" && host != "127.0.0.1" && host != "localhost" {
		return host
	}
	// For localhost, use the full address as slug to distinguish endpoints.
	if address != "" {
		return strings.ReplaceAll(address, ":", "-")
	}
	return uri
}
