package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	if err := run(); err != nil {
		resp := &pluginpb.CodeGeneratorResponse{
			Error: proto.String(err.Error()),
		}
		data, marshalErr := proto.Marshal(resp)
		if marshalErr != nil {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, marshalErr)
			os.Exit(1)
		}
		_, _ = os.Stdout.Write(data)
	}
}

func run() error {
	lang := adapterLang()
	if lang == "" {
		return fmt.Errorf("cannot infer adapter language from %q; pass --lang=<name> or name the binary protoc-gen-<name>", os.Args[0])
	}

	reqBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}
	req := &pluginpb.CodeGeneratorRequest{}
	if err := proto.Unmarshal(reqBytes, req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	if len(req.GetFileToGenerate()) == 0 {
		return writeResponse(&pluginpb.CodeGeneratorResponse{})
	}

	tmp, err := os.MkdirTemp("", "op-protoc-adapter-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	descPath := filepath.Join(tmp, "request.pb")
	fdset := &descriptorpb.FileDescriptorSet{File: req.GetProtoFile()}
	descBytes, err := proto.Marshal(fdset)
	if err != nil {
		return fmt.Errorf("marshal descriptor set: %w", err)
	}
	if err := os.WriteFile(descPath, descBytes, 0o644); err != nil {
		return fmt.Errorf("write descriptor set: %w", err)
	}

	outDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	protoc, err := siblingExecutable("protoc")
	if err != nil {
		return err
	}
	args, err := protocArgs(lang, descPath, outDir)
	if err != nil {
		return err
	}
	args = append(args, req.GetFileToGenerate()...)

	cmd := exec.Command(protoc, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("protoc %s adapter failed: %w", lang, err)
	}

	files, err := collectGeneratedFiles(outDir)
	if err != nil {
		return err
	}
	return writeResponse(&pluginpb.CodeGeneratorResponse{File: files})
}

func adapterLang() string {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--lang=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--lang="))
		}
	}
	if value := strings.TrimSpace(os.Getenv("OP_PROTOC_ADAPTER_LANG")); value != "" {
		return value
	}
	base := filepath.Base(os.Args[0])
	base = strings.TrimSuffix(base, ".exe")
	base = strings.TrimPrefix(base, "protoc-gen-")
	base = strings.TrimPrefix(base, "op-adapter-")
	if base == "op-adapter" || base == "protoc-gen-op-adapter" {
		return ""
	}
	return base
}

func protocArgs(lang, descPath, outDir string) ([]string, error) {
	args := []string{"--descriptor_set_in=" + descPath}
	switch lang {
	case "zig":
		args = append(args, "--c_out="+outDir)
		if plugin, err := optionalSiblingExecutable("protoc-gen-c"); err == nil {
			args = append(args, "--plugin=protoc-gen-c="+plugin)
		}
	case "cpp":
		args = append(args, "--cpp_out="+outDir)
		if plugin, err := optionalSiblingExecutable("grpc_cpp_plugin"); err == nil {
			args = append(args, "--grpc_out="+outDir, "--plugin=protoc-gen-grpc="+plugin)
		}
	case "csharp":
		args = append(args, "--csharp_out="+outDir)
		if plugin, err := optionalSiblingExecutable("grpc_csharp_plugin"); err == nil {
			args = append(args, "--grpc_out="+outDir, "--plugin=protoc-gen-grpc="+plugin)
		}
	case "java":
		args = append(args, "--java_out="+outDir)
		if plugin, err := optionalSiblingExecutable("protoc-gen-grpc-java"); err == nil {
			args = append(args, "--grpc-java_out="+outDir, "--plugin=protoc-gen-grpc-java="+plugin)
		}
	case "js":
		args = append(args, "--js_out=import_style=commonjs,binary:"+outDir)
		if plugin, err := optionalSiblingExecutable("grpc_tools_node_protoc_plugin"); err == nil {
			args = append(args, "--grpc_out=grpc_js:"+outDir, "--plugin=protoc-gen-grpc="+plugin)
		}
	case "kotlin-java":
		args = append(args, "--java_out="+outDir)
	case "kotlin-java-grpc":
		if plugin, err := optionalSiblingExecutable("protoc-gen-grpc-java"); err == nil {
			args = append(args, "--grpc-java_out="+outDir, "--plugin=protoc-gen-grpc-java="+plugin)
		}
	case "kotlin":
		args = append(args, "--kotlin_out="+outDir)
	case "kotlin-grpc":
		if plugin, err := optionalSiblingExecutable("protoc-gen-grpc-kotlin"); err == nil {
			args = append(args, "--grpc-kotlin_out="+outDir, "--plugin=protoc-gen-grpc-kotlin="+plugin)
		}
	case "python":
		args = append(args, "--python_out="+outDir)
		if plugin, err := optionalSiblingExecutable("grpc_python_plugin"); err == nil {
			args = append(args, "--grpc_python_out="+outDir, "--plugin=protoc-gen-grpc_python="+plugin)
		}
	case "ruby":
		args = append(args, "--ruby_out="+outDir)
		if plugin, err := optionalSiblingExecutable("grpc_ruby_plugin"); err == nil {
			args = append(args, "--grpc_out="+outDir, "--plugin=protoc-gen-grpc="+plugin)
		}
	default:
		return nil, fmt.Errorf("unsupported protoc adapter language %q", lang)
	}
	return args, nil
}

func siblingExecutable(name string) (string, error) {
	if path, err := optionalSiblingExecutable(name); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("%s not found next to adapter or on PATH", name)
}

func optionalSiblingExecutable(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	candidates := []string{filepath.Join(dir, name)}
	if !strings.HasSuffix(name, ".exe") {
		candidates = append(candidates, filepath.Join(dir, name+".exe"))
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, nil
	}
	return "", os.ErrNotExist
}

func collectGeneratedFiles(root string) ([]*pluginpb.CodeGeneratorResponse_File, error) {
	var rels []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rels = append(rels, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("collect generated files: %w", err)
	}
	sort.Strings(rels)

	files := make([]*pluginpb.CodeGeneratorResponse_File, 0, len(rels))
	for _, rel := range rels {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("read generated file %s: %w", rel, err)
		}
		content := string(data)
		files = append(files, &pluginpb.CodeGeneratorResponse_File{
			Name:    proto.String(rel),
			Content: proto.String(content),
		})
	}
	return files, nil
}

func writeResponse(resp *pluginpb.CodeGeneratorResponse) error {
	if resp.SupportedFeatures == nil {
		resp.SupportedFeatures = proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL))
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}
