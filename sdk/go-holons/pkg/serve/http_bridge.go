package serve

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/go-holons/pkg/describe"
	"github.com/organic-programming/go-holons/pkg/holonrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

type httpBridge struct {
	unary  map[string]holonrpc.Handler
	stream map[string]holonrpc.StreamHandler
}

func newHTTPBridge(server *grpc.Server, conn *grpc.ClientConn, response *holonsv1.DescribeResponse) (*httpBridge, error) {
	if server == nil {
		return nil, fmt.Errorf("grpc server is required")
	}
	if conn == nil {
		return nil, fmt.Errorf("grpc client connection is required")
	}
	if response == nil {
		return nil, fmt.Errorf("http bridge: %w", describe.ErrNoIncodeDescription)
	}

	bridge := &httpBridge{
		unary:  make(map[string]holonrpc.Handler),
		stream: make(map[string]holonrpc.StreamHandler),
	}

	documented := indexDescribeMethods(response)
	documented["holons.v1.HolonMeta"] = map[string]*holonsv1.MethodDoc{
		"Describe": {
			Name:       "Describe",
			InputType:  "holons.v1.DescribeRequest",
			OutputType: "holons.v1.DescribeResponse",
		},
	}
	documented["holons.v1.HolonObservability"] = map[string]*holonsv1.MethodDoc{
		"Logs": {
			Name:            "Logs",
			InputType:       "holons.v1.LogsRequest",
			OutputType:      "holons.v1.LogEntry",
			ServerStreaming: true,
		},
		"Metrics": {
			Name:       "Metrics",
			InputType:  "holons.v1.MetricsRequest",
			OutputType: "holons.v1.MetricsSnapshot",
		},
		"Events": {
			Name:            "Events",
			InputType:       "holons.v1.EventsRequest",
			OutputType:      "holons.v1.EventInfo",
			ServerStreaming: true,
		},
	}

	for serviceName, info := range server.GetServiceInfo() {
		if strings.HasPrefix(serviceName, "grpc.reflection.") {
			continue
		}

		methods := documented[serviceName]
		for _, methodInfo := range info.Methods {
			fullName := serviceName + "/" + methodInfo.Name
			if methodInfo.IsClientStream {
				log.Printf("HTTP+SSE bridge skipping unsupported client-streaming method %s", fullName)
				continue
			}

			doc := methods[methodInfo.Name]
			if doc == nil {
				return nil, fmt.Errorf("http bridge: %s missing from static DescribeResponse", fullName)
			}

			method, err := newBridgeMethod(serviceName, doc, methodInfo)
			if err != nil {
				return nil, fmt.Errorf("http bridge: %s: %w", fullName, err)
			}

			if methodInfo.IsServerStream {
				bridge.stream[fullName] = method.streamHandler(conn)
				continue
			}
			bridge.unary[fullName] = method.unaryHandler(conn)
		}
	}

	return bridge, nil
}

func (b *httpBridge) apply(server *holonrpc.HTTPServer) {
	if b == nil || server == nil {
		return
	}
	for method, handler := range b.unary {
		server.Register(method, handler)
	}
	for method, handler := range b.stream {
		server.RegisterStream(method, handler)
	}
}

type bridgeMethod struct {
	fullMethod string
	input      messageFactory
	output     messageFactory
}

func newBridgeMethod(serviceName string, doc *holonsv1.MethodDoc, methodInfo grpc.MethodInfo) (*bridgeMethod, error) {
	if doc == nil {
		return nil, fmt.Errorf("method metadata is required")
	}
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if strings.TrimSpace(doc.GetName()) == "" {
		return nil, fmt.Errorf("method name is required")
	}
	if doc.GetClientStreaming() != methodInfo.IsClientStream {
		return nil, fmt.Errorf("client_streaming mismatch between DescribeResponse and registered gRPC method")
	}
	if doc.GetServerStreaming() != methodInfo.IsServerStream {
		return nil, fmt.Errorf("server_streaming mismatch between DescribeResponse and registered gRPC method")
	}

	input, err := resolveMessageFactory(doc.GetInputType())
	if err != nil {
		return nil, fmt.Errorf("resolve input type %q: %w", doc.GetInputType(), err)
	}
	output, err := resolveMessageFactory(doc.GetOutputType())
	if err != nil {
		return nil, fmt.Errorf("resolve output type %q: %w", doc.GetOutputType(), err)
	}

	return &bridgeMethod{
		fullMethod: "/" + serviceName + "/" + doc.GetName(),
		input:      input,
		output:     output,
	}, nil
}

func (m *bridgeMethod) unaryHandler(conn *grpc.ClientConn) holonrpc.Handler {
	return func(ctx context.Context, params map[string]any) (map[string]any, error) {
		request, err := m.input.decode(params)
		if err != nil {
			return nil, err
		}

		response := m.output.newMessage()
		if err := conn.Invoke(ctx, m.fullMethod, request, response); err != nil {
			return nil, grpcToHolonRPCError(err)
		}

		result, err := m.output.encode(response)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
}

func (m *bridgeMethod) streamHandler(conn *grpc.ClientConn) holonrpc.StreamHandler {
	return func(ctx context.Context, params map[string]any, send func(map[string]any) error) error {
		request, err := m.input.decode(params)
		if err != nil {
			return err
		}

		stream, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, m.fullMethod)
		if err != nil {
			return grpcToHolonRPCError(err)
		}
		if err := stream.SendMsg(request); err != nil {
			return grpcToHolonRPCError(err)
		}
		if err := stream.CloseSend(); err != nil {
			return grpcToHolonRPCError(err)
		}

		for {
			response := m.output.newMessage()
			if err := stream.RecvMsg(response); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return grpcToHolonRPCError(err)
			}

			result, err := m.output.encode(response)
			if err != nil {
				return err
			}
			if err := send(result); err != nil {
				return err
			}
		}
	}
}

type messageFactory struct {
	desc       protoreflect.MessageDescriptor
	newMessage func() proto.Message
}

func resolveMessageFactory(typeName string) (messageFactory, error) {
	fullName := protoreflect.FullName(strings.TrimPrefix(strings.TrimSpace(typeName), "."))
	if fullName == "" {
		return messageFactory{}, fmt.Errorf("message type is empty")
	}

	if messageType, err := protoregistry.GlobalTypes.FindMessageByName(fullName); err == nil {
		return messageFactory{
			desc: messageType.Descriptor(),
			newMessage: func() proto.Message {
				return messageType.New().Interface()
			},
		}, nil
	}

	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(fullName)
	if err != nil {
		return messageFactory{}, err
	}
	messageDescriptor, ok := descriptor.(protoreflect.MessageDescriptor)
	if !ok {
		return messageFactory{}, fmt.Errorf("%s is not a message type", fullName)
	}

	messageType := dynamicpb.NewMessageType(messageDescriptor)
	return messageFactory{
		desc: messageDescriptor,
		newMessage: func() proto.Message {
			return messageType.New().Interface()
		},
	}, nil
}

func (f messageFactory) decode(params map[string]any) (proto.Message, error) {
	if params == nil {
		params = map[string]any{}
	}

	normalized, err := normalizeParams(params, f.desc)
	if err != nil {
		return nil, invalidParamsError(err)
	}

	raw, err := json.Marshal(normalized)
	if err != nil {
		return nil, invalidParamsError(err)
	}

	message := f.newMessage()
	if err := protojson.Unmarshal(raw, message); err != nil {
		return nil, invalidParamsError(err)
	}
	return message, nil
}

func (f messageFactory) encode(message proto.Message) (map[string]any, error) {
	if message == nil {
		return map[string]any{}, nil
	}

	raw, err := protojson.Marshal(message)
	if err != nil {
		return nil, &holonrpc.ResponseError{Code: int(codes.Internal), Message: "failed to encode gRPC response"}
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return map[string]any{}, nil
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, &holonrpc.ResponseError{Code: int(codes.Internal), Message: "failed to decode gRPC response"}
	}
	if out == nil {
		return map[string]any{}, nil
	}
	return out, nil
}

func indexDescribeMethods(response *holonsv1.DescribeResponse) map[string]map[string]*holonsv1.MethodDoc {
	index := make(map[string]map[string]*holonsv1.MethodDoc, len(response.GetServices()))
	for _, service := range response.GetServices() {
		if service == nil || strings.TrimSpace(service.GetName()) == "" {
			continue
		}
		methods := make(map[string]*holonsv1.MethodDoc, len(service.GetMethods()))
		for _, method := range service.GetMethods() {
			if method == nil || strings.TrimSpace(method.GetName()) == "" {
				continue
			}
			methods[method.GetName()] = method
		}
		index[service.GetName()] = methods
	}
	return index
}

func normalizeParams(params map[string]any, desc protoreflect.MessageDescriptor) (map[string]any, error) {
	if len(params) == 0 || desc == nil {
		return params, nil
	}

	normalized := make(map[string]any, len(params))
	fields := desc.Fields()
	for key, value := range params {
		field := fields.ByJSONName(key)
		if field == nil {
			field = fields.ByTextName(key)
		}
		if field == nil {
			normalized[key] = value
			continue
		}

		coerced, err := coerceFieldValue(field, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		normalized[key] = coerced
	}
	return normalized, nil
}

func coerceFieldValue(field protoreflect.FieldDescriptor, value any) (any, error) {
	if field.IsMap() {
		return value, nil
	}
	if field.IsList() {
		switch items := value.(type) {
		case []string:
			out := make([]any, 0, len(items))
			for _, item := range items {
				coerced, err := coerceListItem(field, item)
				if err != nil {
					return nil, err
				}
				out = append(out, coerced)
			}
			return out, nil
		case []any:
			out := make([]any, 0, len(items))
			for _, item := range items {
				coerced, err := coerceListItem(field, item)
				if err != nil {
					return nil, err
				}
				out = append(out, coerced)
			}
			return out, nil
		default:
			return value, nil
		}
	}
	if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
		params, ok := value.(map[string]any)
		if !ok {
			return value, nil
		}
		return normalizeParams(params, field.Message())
	}
	return coerceScalarValue(field, value)
}

func coerceListItem(field protoreflect.FieldDescriptor, value any) (any, error) {
	if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
		params, ok := value.(map[string]any)
		if !ok {
			return value, nil
		}
		return normalizeParams(params, field.Message())
	}
	return coerceScalarValue(field, value)
}

func coerceScalarValue(field protoreflect.FieldDescriptor, value any) (any, error) {
	raw, ok := value.(string)
	if !ok {
		return value, nil
	}

	switch field.Kind() {
	case protoreflect.BoolKind:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case protoreflect.EnumKind:
		if number, err := strconv.ParseInt(raw, 10, 32); err == nil {
			return number, nil
		}
		return raw, nil
	case protoreflect.BytesKind:
		if _, err := base64.StdEncoding.DecodeString(raw); err == nil {
			return raw, nil
		}
		return base64.StdEncoding.EncodeToString([]byte(raw)), nil
	default:
		return value, nil
	}
}

func invalidParamsError(err error) error {
	if err == nil {
		return nil
	}
	return &holonrpc.ResponseError{
		Code:    -32602,
		Message: "invalid params",
		Data:    err.Error(),
	}
}

func grpcToHolonRPCError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return &holonrpc.ResponseError{Code: int(codes.Internal), Message: err.Error()}
	}
	return &holonrpc.ResponseError{Code: int(st.Code()), Message: st.Message()}
}
