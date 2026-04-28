package holons

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"github.com/organic-programming/grace-op/internal/progress"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

type cServeMethod struct {
	ServiceName     string
	ServiceSymbol   string
	MethodName      string
	HandlerField    string
	FullMethodPath  string
	InputSymbol     string
	OutputSymbol    string
	InputHeader     string
	OutputHeader    string
	InputProtoFile  string
	OutputProtoFile string
}

func generateServeSource(manifest *LoadedManifest, reporter progress.Reporter) (restore func(), err error) {
	restore = func() {}

	if manifest == nil || strings.TrimSpace(manifest.Manifest.Lang) != "c" {
		return restore, nil
	}

	response, err := buildDescribeResponse(manifest)
	if err != nil {
		return restore, fmt.Errorf("build describe response: %w", err)
	}

	fds, err := loadDescriptorFiles(filepath.Join(manifest.OpRoot(), "pb", "descriptors.binpb"))
	if err != nil {
		return restore, fmt.Errorf("load descriptor set: %w", err)
	}

	header, source, err := renderCServeSource(manifest, response, fds)
	if err != nil {
		return restore, err
	}

	headerPath := filepath.Join(manifest.Dir, "gen", "serve_generated.h")
	sourcePath := filepath.Join(manifest.Dir, "gen", "serve_generated.c")

	restoreHeader, err := writeGeneratedFile(headerPath, header)
	if err != nil {
		return restore, err
	}
	restoreSource, err := writeGeneratedFile(sourcePath, source)
	if err != nil {
		restoreHeader()
		return restore, err
	}

	restore = func() {
		restoreSource()
		restoreHeader()
	}

	reporter.Step(fmt.Sprintf("native serve: %s", workspaceRelativePath(headerPath)))
	reporter.Step(fmt.Sprintf("native serve: %s", workspaceRelativePath(sourcePath)))
	return restore, nil
}

func loadDescriptorFiles(path string) (map[string]protoreflect.FileDescriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fdSet descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fdSet); err != nil {
		return nil, fmt.Errorf("unmarshal descriptor set: %w", err)
	}

	files, err := protodesc.NewFiles(&fdSet)
	if err != nil {
		return nil, fmt.Errorf("create file descriptors: %w", err)
	}
	out := make(map[string]protoreflect.FileDescriptor)
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		out[fd.Path()] = fd
		return true
	})
	return out, nil
}

func renderCServeSource(manifest *LoadedManifest, _ *holonsv1.DescribeResponse, files map[string]protoreflect.FileDescriptor) ([]byte, []byte, error) {
	methods, err := collectCServeMethods(files)
	if err != nil {
		return nil, nil, err
	}

	header, err := renderCServeHeader(manifest, methods)
	if err != nil {
		return nil, nil, err
	}
	source, err := renderCServeImplementation(manifest, methods)
	if err != nil {
		return nil, nil, err
	}
	return header, source, nil
}

func collectCServeMethods(files map[string]protoreflect.FileDescriptor) ([]cServeMethod, error) {
	if len(files) == 0 {
		return nil, nil
	}

	var serviceDescriptors []protoreflect.ServiceDescriptor
	for _, fd := range files {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			serviceDescriptors = append(serviceDescriptors, services.Get(i))
		}
	}
	sort.Slice(serviceDescriptors, func(i, j int) bool {
		return serviceDescriptors[i].FullName() < serviceDescriptors[j].FullName()
	})

	nameCounts := map[string]int{}
	for _, service := range serviceDescriptors {
		if service.FullName() == "holons.v1.HolonMeta" {
			continue
		}
		methods := service.Methods()
		for i := 0; i < methods.Len(); i++ {
			nameCounts[sanitizeCIdentifier(string(methods.Get(i).Name()))]++
		}
	}

	methods := make([]cServeMethod, 0)
	for _, service := range serviceDescriptors {
		if service.FullName() == "holons.v1.HolonMeta" {
			continue
		}

		serviceSymbol := sanitizeCIdentifier(string(service.Name()))
		serviceMethods := service.Methods()
		for i := 0; i < serviceMethods.Len(); i++ {
			method := serviceMethods.Get(i)
			if method.IsStreamingClient() || method.IsStreamingServer() {
				return nil, fmt.Errorf("c native serve does not support streaming RPC %s/%s", service.FullName(), method.Name())
			}

			handlerField := sanitizeCIdentifier(string(method.Name()))
			if nameCounts[handlerField] > 1 {
				handlerField = serviceSymbol + "_" + handlerField
			}

			methods = append(methods, cServeMethod{
				ServiceName:     string(service.Name()),
				ServiceSymbol:   serviceSymbol,
				MethodName:      string(method.Name()),
				HandlerField:    handlerField,
				FullMethodPath:  "/" + string(service.FullName()) + "/" + string(method.Name()),
				InputSymbol:     cProtoSymbol(string(method.Input().FullName())),
				OutputSymbol:    cProtoSymbol(string(method.Output().FullName())),
				InputHeader:     cUPBHeaderName(method.Input()),
				OutputHeader:    cUPBHeaderName(method.Output()),
				InputProtoFile:  method.Input().ParentFile().Path(),
				OutputProtoFile: method.Output().ParentFile().Path(),
			})
		}
	}

	return methods, nil
}

func renderCServeHeader(manifest *LoadedManifest, methods []cServeMethod) ([]byte, error) {
	prefix := sanitizeCIdentifier(manifest.Name)
	guard := strings.ToUpper(prefix) + "_SERVE_GENERATED_H"

	typeSymbols := sortedUniqueSymbols(methods)
	var buf bytes.Buffer
	buf.WriteString("// Code generated by op build. DO NOT EDIT.\n\n")
	buf.WriteString("#ifndef " + guard + "\n")
	buf.WriteString("#define " + guard + "\n\n")
	buf.WriteString("#include <stddef.h>\n\n")
	buf.WriteString("#include \"holons/holons.h\"\n")
	buf.WriteString("#include \"upb/mem/arena.h\"\n\n")
	buf.WriteString("#ifdef __cplusplus\nextern \"C\" {\n#endif\n\n")

	for _, symbol := range typeSymbols {
		buf.WriteString("typedef struct " + symbol + " " + symbol + ";\n")
	}
	if len(typeSymbols) > 0 {
		buf.WriteString("\n")
	}

	buf.WriteString("typedef struct " + prefix + "_handlers {\n")
	buf.WriteString("  void *ctx;\n")
	for _, method := range methods {
		buf.WriteString(fmt.Sprintf("  %s *(*%s)(const %s *request, upb_Arena *arena, void *ctx);\n",
			method.OutputSymbol,
			method.HandlerField,
			method.InputSymbol,
		))
	}
	buf.WriteString("} " + prefix + "_handlers_t;\n\n")

	buf.WriteString(fmt.Sprintf("int %s_generated_serve(const char *listen_uri,\n", prefix))
	buf.WriteString(fmt.Sprintf("                             const %s_handlers_t *handlers,\n", prefix))
	buf.WriteString("                             const holons_grpc_serve_options_t *options,\n")
	buf.WriteString("                             char *err,\n")
	buf.WriteString("                             size_t err_len);\n\n")

	buf.WriteString("#ifdef __cplusplus\n}\n#endif\n\n")
	buf.WriteString("#endif\n")
	return buf.Bytes(), nil
}

func renderCServeImplementation(manifest *LoadedManifest, methods []cServeMethod) ([]byte, error) {
	prefix := sanitizeCIdentifier(manifest.Name)
	headerIncludes, err := cServeHeaderIncludes(methods)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString("// Code generated by op build. DO NOT EDIT.\n\n")
	buf.WriteString("#include \"gen/serve_generated.h\"\n\n")
	for _, include := range headerIncludes {
		buf.WriteString(fmt.Sprintf("#include %q\n", include))
	}
	if len(headerIncludes) > 0 {
		buf.WriteString("\n")
	}
	buf.WriteString("#include <stdlib.h>\n")
	buf.WriteString("#include <string.h>\n")
	buf.WriteString("#include <stdio.h>\n\n")

	buf.WriteString("const unsigned char *holons_generated_describe_response_bytes(size_t *len);\n\n")

	buf.WriteString("static void holons_generated_set_err(char *err, size_t err_len, const char *message) {\n")
	buf.WriteString("  if (err == NULL || err_len == 0) {\n")
	buf.WriteString("    return;\n")
	buf.WriteString("  }\n")
	buf.WriteString("  snprintf(err, err_len, \"%s\", message != NULL ? message : \"unknown error\");\n")
	buf.WriteString("}\n\n")

	buf.WriteString("static int holons_generated_copy_response(const unsigned char *src,\n")
	buf.WriteString("                                          size_t src_len,\n")
	buf.WriteString("                                          unsigned char **out,\n")
	buf.WriteString("                                          size_t *out_len,\n")
	buf.WriteString("                                          char *err,\n")
	buf.WriteString("                                          size_t err_len) {\n")
	buf.WriteString("  unsigned char *copy = NULL;\n")
	buf.WriteString("  if (out == NULL || out_len == NULL) {\n")
	buf.WriteString("    holons_generated_set_err(err, err_len, \"response output buffers are required\");\n")
	buf.WriteString("    return -1;\n")
	buf.WriteString("  }\n")
	buf.WriteString("  *out = NULL;\n")
	buf.WriteString("  *out_len = src_len;\n")
	buf.WriteString("  if (src_len == 0) {\n")
	buf.WriteString("    return 0;\n")
	buf.WriteString("  }\n")
	buf.WriteString("  copy = (unsigned char *)malloc(src_len);\n")
	buf.WriteString("  if (copy == NULL) {\n")
	buf.WriteString("    holons_generated_set_err(err, err_len, \"out of memory\");\n")
	buf.WriteString("    return -1;\n")
	buf.WriteString("  }\n")
	buf.WriteString("  memcpy(copy, src, src_len);\n")
	buf.WriteString("  *out = copy;\n")
	buf.WriteString("  return 0;\n")
	buf.WriteString("}\n\n")

	buf.WriteString("static int holons_generated_handle_describe(const unsigned char *request_data,\n")
	buf.WriteString("                                            size_t request_len,\n")
	buf.WriteString("                                            void *ctx,\n")
	buf.WriteString("                                            unsigned char **response_data,\n")
	buf.WriteString("                                            size_t *response_len,\n")
	buf.WriteString("                                            char *err,\n")
	buf.WriteString("                                            size_t err_len) {\n")
	buf.WriteString("  const unsigned char *bytes;\n")
	buf.WriteString("  size_t bytes_len = 0;\n")
	buf.WriteString("  (void)request_data;\n")
	buf.WriteString("  (void)request_len;\n")
	buf.WriteString("  (void)ctx;\n")
	buf.WriteString("  bytes = holons_generated_describe_response_bytes(&bytes_len);\n")
	buf.WriteString("  if (bytes == NULL) {\n")
	buf.WriteString("    holons_generated_set_err(err, err_len, \"describe response bytes are not available\");\n")
	buf.WriteString("    return -1;\n")
	buf.WriteString("  }\n")
	buf.WriteString("  return holons_generated_copy_response(bytes, bytes_len, response_data, response_len, err, err_len);\n")
	buf.WriteString("}\n\n")

	for _, method := range methods {
		buf.WriteString(fmt.Sprintf("static int %s_handle_%s_raw(const unsigned char *request_data,\n", prefix, method.HandlerField))
		buf.WriteString("                                        size_t request_len,\n")
		buf.WriteString("                                        void *ctx,\n")
		buf.WriteString("                                        unsigned char **response_data,\n")
		buf.WriteString("                                        size_t *response_len,\n")
		buf.WriteString("                                        char *err,\n")
		buf.WriteString("                                        size_t err_len) {\n")
		buf.WriteString(fmt.Sprintf("  const %s_handlers_t *handlers = (const %s_handlers_t *)ctx;\n", prefix, prefix))
		buf.WriteString("  upb_Arena *arena;\n")
		buf.WriteString(fmt.Sprintf("  %s *request;\n", method.InputSymbol))
		buf.WriteString(fmt.Sprintf("  %s *response;\n", method.OutputSymbol))
		buf.WriteString("  char *serialized = NULL;\n")
		buf.WriteString("  size_t serialized_len = 0;\n\n")
		buf.WriteString("  if (handlers == NULL) {\n")
		buf.WriteString("    holons_generated_set_err(err, err_len, \"generated handlers are required\");\n")
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n")
		buf.WriteString(fmt.Sprintf("  if (handlers->%s == NULL) {\n", method.HandlerField))
		buf.WriteString(fmt.Sprintf("    holons_generated_set_err(err, err_len, \"handler missing for %s/%s\");\n", method.ServiceName, method.MethodName))
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n\n")
		buf.WriteString("  arena = upb_Arena_New();\n")
		buf.WriteString("  if (arena == NULL) {\n")
		buf.WriteString("    holons_generated_set_err(err, err_len, \"failed to allocate upb arena\");\n")
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n")
		buf.WriteString(fmt.Sprintf("  request = %s_parse((const char *)request_data, request_len, arena);\n", method.InputSymbol))
		buf.WriteString("  if (request == NULL) {\n")
		buf.WriteString(fmt.Sprintf("    holons_generated_set_err(err, err_len, \"failed to parse request for %s/%s\");\n", method.ServiceName, method.MethodName))
		buf.WriteString("    upb_Arena_Free(arena);\n")
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n")
		buf.WriteString(fmt.Sprintf("  response = handlers->%s(request, arena, handlers->ctx);\n", method.HandlerField))
		buf.WriteString("  if (response == NULL) {\n")
		buf.WriteString(fmt.Sprintf("    holons_generated_set_err(err, err_len, \"native handler returned no response for %s/%s\");\n", method.ServiceName, method.MethodName))
		buf.WriteString("    upb_Arena_Free(arena);\n")
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n")
		buf.WriteString(fmt.Sprintf("  serialized = %s_serialize(response, arena, &serialized_len);\n", method.OutputSymbol))
		buf.WriteString("  if (serialized == NULL && serialized_len > 0) {\n")
		buf.WriteString(fmt.Sprintf("    holons_generated_set_err(err, err_len, \"failed to serialize response for %s/%s\");\n", method.ServiceName, method.MethodName))
		buf.WriteString("    upb_Arena_Free(arena);\n")
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n")
		buf.WriteString("  if (holons_generated_copy_response((const unsigned char *)serialized, serialized_len, response_data, response_len, err, err_len) != 0) {\n")
		buf.WriteString("    upb_Arena_Free(arena);\n")
		buf.WriteString("    return -1;\n")
		buf.WriteString("  }\n")
		buf.WriteString("  upb_Arena_Free(arena);\n")
		buf.WriteString("  return 0;\n")
		buf.WriteString("}\n\n")
	}

	buf.WriteString(fmt.Sprintf("int %s_generated_serve(const char *listen_uri,\n", prefix))
	buf.WriteString(fmt.Sprintf("                             const %s_handlers_t *handlers,\n", prefix))
	buf.WriteString("                             const holons_grpc_serve_options_t *options,\n")
	buf.WriteString("                             char *err,\n")
	buf.WriteString("                             size_t err_len) {\n")
	buf.WriteString("  holons_grpc_serve_options_t defaults;\n")
	buf.WriteString("  const holons_grpc_serve_options_t *effective = options;\n")
	buf.WriteString(fmt.Sprintf("  holons_grpc_unary_registration_t registrations[%d];\n", len(methods)+1))
	buf.WriteString("  size_t registration_count = 0;\n\n")
	buf.WriteString("  if (handlers == NULL) {\n")
	buf.WriteString("    holons_generated_set_err(err, err_len, \"generated handlers are required\");\n")
	buf.WriteString("    return -1;\n")
	buf.WriteString("  }\n\n")
	buf.WriteString("  defaults.announce = 1;\n")
	buf.WriteString("  defaults.enable_reflection = 0;\n")
	buf.WriteString("  defaults.graceful_shutdown_timeout_ms = 10000;\n")
	buf.WriteString("  if (effective == NULL) {\n")
	buf.WriteString("    effective = &defaults;\n")
	buf.WriteString("  }\n\n")
	buf.WriteString("  registrations[registration_count].full_method = \"/holons.v1.HolonMeta/Describe\";\n")
	buf.WriteString("  registrations[registration_count].handler = holons_generated_handle_describe;\n")
	buf.WriteString("  registrations[registration_count].ctx = NULL;\n")
	buf.WriteString("  registration_count++;\n")
	for _, method := range methods {
		buf.WriteString(fmt.Sprintf("  registrations[registration_count].full_method = %s;\n", strconv.Quote(method.FullMethodPath)))
		buf.WriteString(fmt.Sprintf("  registrations[registration_count].handler = %s_handle_%s_raw;\n", prefix, method.HandlerField))
		buf.WriteString("  registrations[registration_count].ctx = (void *)handlers;\n")
		buf.WriteString("  registration_count++;\n")
	}
	buf.WriteString("\n  return holons_serve_grpc(listen_uri, registrations, registration_count, effective, err, err_len);\n")
	buf.WriteString("}\n")
	return buf.Bytes(), nil
}

func writeGeneratedFile(path string, data []byte) (restore func(), err error) {
	restore = func() {}

	originalExists := false
	var original []byte
	mode := os.FileMode(0o644)

	if info, statErr := os.Stat(path); statErr == nil {
		originalExists = true
		mode = info.Mode()
		original, err = os.ReadFile(path)
		if err != nil {
			return func() {}, fmt.Errorf("read existing output %s: %w", path, err)
		}
	} else if !os.IsNotExist(statErr) {
		return func() {}, fmt.Errorf("stat output %s: %w", path, statErr)
	}

	restore = func() {
		if originalExists {
			_ = os.WriteFile(path, original, mode)
			return
		}
		_ = os.Remove(path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return func() {}, fmt.Errorf("create output dir for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		restore()
		return func() {}, fmt.Errorf("write %s: %w", path, err)
	}
	return restore, nil
}

func sortedUniqueSymbols(methods []cServeMethod) []string {
	seen := map[string]bool{}
	var symbols []string
	for _, method := range methods {
		for _, symbol := range []string{method.InputSymbol, method.OutputSymbol} {
			if symbol == "" || seen[symbol] {
				continue
			}
			seen[symbol] = true
			symbols = append(symbols, symbol)
		}
	}
	sort.Strings(symbols)
	return symbols
}

func sanitizeCIdentifier(raw string) string {
	var buf strings.Builder
	for i, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			buf.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			if i == 0 {
				buf.WriteRune(r + ('a' - 'A'))
			} else {
				buf.WriteRune(r)
			}
		case r >= '0' && r <= '9':
			if i == 0 {
				buf.WriteByte('_')
			}
			buf.WriteRune(r)
		default:
			buf.WriteByte('_')
		}
	}
	result := strings.Trim(buf.String(), "_")
	if result == "" {
		return "generated"
	}
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return result
}

func cProtoSymbol(fullName string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(fullName), ".")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ".")
	return strings.Join(parts, "_")
}

func cUPBHeaderName(message protoreflect.MessageDescriptor) string {
	if message == nil || message.ParentFile() == nil {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(message.ParentFile().Path()), filepath.Ext(message.ParentFile().Path()))
	if base == "" {
		return ""
	}
	return base + ".upb.h"
}

func cServeHeaderIncludes(methods []cServeMethod) ([]string, error) {
	includeToProto := map[string]string{}
	var includes []string
	add := func(include, protoFile string) error {
		if include == "" {
			return nil
		}
		if existing, ok := includeToProto[include]; ok {
			if existing != protoFile {
				return fmt.Errorf("c native serve cannot disambiguate generated upb header %q for %s and %s", include, existing, protoFile)
			}
			return nil
		}
		includeToProto[include] = protoFile
		includes = append(includes, include)
		return nil
	}
	for _, method := range methods {
		if err := add(method.InputHeader, method.InputProtoFile); err != nil {
			return nil, err
		}
		if err := add(method.OutputHeader, method.OutputProtoFile); err != nil {
			return nil, err
		}
	}
	sort.Strings(includes)
	return includes, nil
}
