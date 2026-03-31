package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	sdkconnect "github.com/organic-programming/go-holons/pkg/connect"
	sdkdiscover "github.com/organic-programming/go-holons/pkg/discover"
	dopkg "github.com/organic-programming/grace-op/internal/do"
	grpcclientpkg "github.com/organic-programming/grace-op/internal/grpcclient"
	"github.com/organic-programming/grace-op/internal/holons"
	toolspkg "github.com/organic-programming/grace-op/internal/tools"
)

const protocolVersion = "2025-06-18"

type Server struct {
	version string
	tools   []toolspkg.Definition
	prompts []promptDefinition

	toolIndex     map[string]toolBinding
	promptIndex   map[string]promptDefinition
	connCache     map[string]sdkconnect.ConnectResult
	describeCache map[string]*holonsv1.DescribeResponse
}

type toolBinding struct {
	slug            string
	definition      toolspkg.Definition
	fullMethod      string
	clientStreaming bool
	serverStreaming bool
	sequence        *holonsv1.HolonManifest_Sequence
}

type promptDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []promptArgument `json:"arguments,omitempty"`
	Text        string           `json:"-"`
}

type promptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewServer connects to one or more holons via Describe and prepares MCP
// tool/prompt state from the runtime schema.
func NewServer(slugs []string, version string) (*Server, error) {
	if len(slugs) == 0 {
		return nil, fmt.Errorf("at least one <slug> is required")
	}

	connCache := make(map[string]sdkconnect.ConnectResult, len(slugs))
	describeCache := make(map[string]*holonsv1.DescribeResponse, len(slugs))
	definitions := make([]toolspkg.Definition, 0)
	toolIndex := make(map[string]toolBinding)
	toolNamesBySlug := make(map[string][]string, len(slugs))

	for _, slug := range slugs {
		result := holons.ConnectRef(slug, nil, sdkdiscover.ALL, int((10*time.Second)/time.Millisecond))
		if result.Error != "" {
			_ = closeConnections(connCache)
			return nil, fmt.Errorf(result.Error)
		}
		connCache[slug] = result

		describeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		response, err := holonsv1.NewHolonMetaClient(result.Channel).Describe(describeCtx, &holonsv1.DescribeRequest{})
		cancel()
		if err != nil {
			_ = closeConnections(connCache)
			return nil, fmt.Errorf("describe %s: %w", slug, err)
		}

		describeCache[slug] = response

		slugDefinitions := toolspkg.DefinitionsFromDescribe(slug, response)
		definitionIndex := make(map[string]toolspkg.Definition, len(slugDefinitions))
		for _, definition := range slugDefinitions {
			definitions = append(definitions, definition)
			definitionIndex[definition.Name] = definition
			toolNamesBySlug[slug] = append(toolNamesBySlug[slug], definition.Name)
		}

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

				name := strings.TrimSpace(slug) + "." + serviceName + "." + methodName
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

			name := strings.TrimSpace(slug) + ".sequence." + sequenceName
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
	}

	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})

	prompts := buildPromptDefinitions(slugs, describeCache, toolNamesBySlug)
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

// Close releases cached holon connections.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	err := closeConnections(s.connCache)
	s.connCache = nil
	return err
}

// ServeStdio runs the MCP server over newline-delimited JSON-RPC on stdio.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	writer := bufio.NewWriter(out)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return err
		}

		line = bytesTrimSpace(line)
		if len(line) > 0 {
			response, hasResponse := s.handle(ctx, line)
			if hasResponse {
				payload, marshalErr := json.Marshal(response)
				if marshalErr != nil {
					return marshalErr
				}
				if _, writeErr := writer.Write(payload); writeErr != nil {
					return writeErr
				}
				if writeErr := writer.WriteByte('\n'); writeErr != nil {
					return writeErr
				}
				if flushErr := writer.Flush(); flushErr != nil {
					return flushErr
				}
			}
		}

		if err == io.EOF {
			return nil
		}
	}
}

func (s *Server) handle(ctx context.Context, payload []byte) (rpcResponse, bool) {
	var request rpcRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      json.RawMessage("null"),
			Error:   &rpcError{Code: -32700, Message: "parse error"},
		}, true
	}

	if len(request.ID) == 0 {
		if request.Method == "notifications/initialized" {
			return rpcResponse{}, false
		}
		return rpcResponse{}, false
	}

	response := rpcResponse{
		JSONRPC: "2.0",
		ID:      request.ID,
	}

	switch request.Method {
	case "initialize":
		response.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]any{
				"tools":   map[string]any{},
				"prompts": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "op",
				"version": strings.TrimSpace(s.version),
			},
			"tools":   s.tools,
			"prompts": s.promptSummaries(),
		}
	case "ping":
		response.Result = map[string]any{}
	case "tools/list":
		response.Result = map[string]any{"tools": s.tools}
	case "tools/call":
		result, err := s.handleToolCall(ctx, request.Params)
		if err != nil {
			response.Error = &rpcError{Code: -32602, Message: err.Error()}
			return response, true
		}
		response.Result = result
	case "prompts/list":
		response.Result = map[string]any{"prompts": s.promptSummaries()}
	case "prompts/get":
		result, err := s.handlePromptGet(request.Params)
		if err != nil {
			response.Error = &rpcError{Code: -32602, Message: err.Error()}
			return response, true
		}
		response.Result = result
	default:
		response.Error = &rpcError{Code: -32601, Message: "method not found"}
	}

	return response, true
}

func (s *Server) handleToolCall(ctx context.Context, params json.RawMessage) (map[string]any, error) {
	var request struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, fmt.Errorf("invalid tools/call params: %w", err)
	}

	binding, ok := s.toolIndex[request.Name]
	if !ok {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: fmt.Sprintf("unknown tool %q", request.Name),
			}},
			"isError": true,
		}, nil
	}

	if binding.sequence != nil {
		return s.handleSequenceCall(request.Name, binding, request.Arguments)
	}

	if strings.TrimSpace(binding.fullMethod) == "" {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: fmt.Sprintf("tool %q is not bound to a callable target", request.Name),
			}},
			"isError": true,
		}, nil
	}

	if binding.clientStreaming || binding.serverStreaming {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: fmt.Sprintf("streaming RPC %q is not supported by op mcp", request.Name),
			}},
			"isError": true,
		}, nil
	}

	args, err := json.Marshal(request.Arguments)
	if err != nil || string(args) == "null" {
		args = []byte("{}")
	}

	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	connected := s.connCache[binding.slug]
	if connected.Channel == nil {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: fmt.Sprintf("connect %s: no cached connection", binding.slug),
			}},
			"isError": true,
		}, nil
	}

	result, err := grpcclientpkg.InvokeConn(callCtx, connected.Channel, binding.fullMethod, string(args))
	if err != nil {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: fmt.Sprintf("call %s failed: %v", request.Name, err),
			}},
			"isError": true,
		}, nil
	}

	var structured map[string]any
	if err := json.Unmarshal([]byte(result.Output), &structured); err != nil {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: result.Output,
			}},
		}, nil
	}

	return map[string]any{
		"content": []textContent{{
			Type: "text",
			Text: result.Output,
		}},
		"structuredContent": structured,
	}, nil
}

func (s *Server) handlePromptGet(params json.RawMessage) (map[string]any, error) {
	var request struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, fmt.Errorf("invalid prompts/get params: %w", err)
	}

	prompt, ok := s.promptIndex[request.Name]
	if !ok {
		return nil, fmt.Errorf("unknown prompt %q", request.Name)
	}

	return map[string]any{
		"description": prompt.Description,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": textContent{
					Type: "text",
					Text: prompt.Text,
				},
			},
		},
	}, nil
}

func (s *Server) promptSummaries() []promptDefinition {
	out := make([]promptDefinition, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		out = append(out, promptDefinition{
			Name:        prompt.Name,
			Description: prompt.Description,
			Arguments:   prompt.Arguments,
		})
	}
	return out
}

func buildPromptDefinitions(
	slugs []string,
	responses map[string]*holonsv1.DescribeResponse,
	toolNamesBySlug map[string][]string,
) []promptDefinition {
	out := make([]promptDefinition, 0)
	for _, slug := range slugs {
		response := responses[slug]
		if response == nil {
			continue
		}

		toolNames := append([]string(nil), toolNamesBySlug[slug]...)
		sort.Strings(toolNames)

		for _, skill := range response.GetManifest().GetSkills() {
			var text strings.Builder
			fmt.Fprintf(&text, "Holon: %s\n", slug)
			if strings.TrimSpace(skill.Description) != "" {
				fmt.Fprintf(&text, "Goal: %s\n", strings.TrimSpace(skill.Description))
			}
			if strings.TrimSpace(skill.When) != "" {
				fmt.Fprintf(&text, "When: %s\n", strings.TrimSpace(skill.When))
			}
			if len(skill.Steps) > 0 {
				text.WriteString("Steps:\n")
				for i, step := range skill.Steps {
					fmt.Fprintf(&text, "%d. %s\n", i+1, strings.TrimSpace(step))
				}
			}
			if len(toolNames) > 0 {
				text.WriteString("Available tools:\n")
				for _, name := range toolNames {
					fmt.Fprintf(&text, "- %s\n", name)
				}
			}

			out = append(out, promptDefinition{
				Name:        slug + "." + skill.GetName(),
				Description: skill.GetDescription(),
				Text:        strings.TrimSpace(text.String()),
			})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func bytesTrimSpace(data []byte) []byte {
	return []byte(strings.TrimSpace(string(data)))
}

func closeConnections(connCache map[string]sdkconnect.ConnectResult) error {
	if len(connCache) == 0 {
		return nil
	}

	errs := make([]error, 0)
	for slug, result := range connCache {
		if result.Channel == nil {
			continue
		}
		if err := sdkconnect.Disconnect(result); err != nil {
			errs = append(errs, fmt.Errorf("disconnect %s: %w", slug, err))
		}
	}
	return errors.Join(errs...)
}

func shortName(name string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(name), ".")
	if trimmed == "" {
		return ""
	}
	if idx := strings.LastIndex(trimmed, "."); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func (s *Server) handleSequenceCall(name string, binding toolBinding, args map[string]any) (map[string]any, error) {
	params := make(map[string]string, len(args))
	for key, value := range args {
		params[key] = stringifyArgument(value)
	}

	result, err := dopkg.Run(binding.slug, binding.sequence.Name, dopkg.Options{
		Params: params,
	})

	payload := any(result)
	isError := false
	if err != nil {
		isError = true
		payload = struct {
			*dopkg.Result
			Error string `json:"error"`
		}{
			Result: result,
			Error:  err.Error(),
		}
	}

	body, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: fmt.Sprintf("marshal response for %s failed: %v", name, marshalErr),
			}},
			"isError": true,
		}, nil
	}

	var structured map[string]any
	if err := json.Unmarshal(body, &structured); err != nil {
		return map[string]any{
			"content": []textContent{{
				Type: "text",
				Text: string(body),
			}},
			"isError": isError,
		}, nil
	}

	pretty, _ := json.MarshalIndent(structured, "", "  ")
	response := map[string]any{
		"content": []textContent{{
			Type: "text",
			Text: string(pretty),
		}},
		"structuredContent": structured,
	}
	if isError {
		response["isError"] = true
	}
	return response, nil
}

func stringifyArgument(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}
