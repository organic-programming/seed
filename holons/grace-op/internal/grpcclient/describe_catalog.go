package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

var (
	errDescribeUnavailable  = errors.New("HolonMeta.Describe unavailable")
	errDescribeInsufficient = errors.New("HolonMeta.Describe response is insufficient")
)

var describeCatalogCache sync.Map

type describeCacheEntry struct {
	mu      sync.Mutex
	ready   bool
	catalog *describeCatalog
	err     error
}

type describeCatalog struct {
	methods []*describeMethod
	byExact map[string]*describeMethod
	byName  map[string][]*describeMethod
}

type describeMethod struct {
	serviceName string
	methodDoc   *holonsv1.MethodDoc
	fullMethod  string

	mu         sync.Mutex
	ready      bool
	inputDesc  protoreflect.MessageDescriptor
	outputDesc protoreflect.MessageDescriptor
	buildErr   error
}

func invokeViaDescribe(ctx context.Context, conn *grpc.ClientConn, methodName string, inputJSON string) (*CallResult, error) {
	catalog, err := getDescribeCatalog(ctx, conn)
	if err != nil {
		return nil, err
	}

	method, err := catalog.resolve(methodName)
	if err != nil {
		return nil, err
	}
	if method.methodDoc.GetClientStreaming() || method.methodDoc.GetServerStreaming() {
		return nil, fmt.Errorf("streaming RPC %q is not supported by op grpc", strings.TrimPrefix(method.fullMethod, "/"))
	}

	inputDesc, outputDesc, err := method.descriptors()
	if err != nil {
		return nil, err
	}

	inputMsg := dynamicpb.NewMessage(inputDesc)
	trimmed := strings.TrimSpace(inputJSON)
	if trimmed == "" {
		trimmed = "{}"
	}
	if err := protojson.Unmarshal([]byte(trimmed), inputMsg); err != nil {
		return nil, fmt.Errorf("parse input JSON: %w", err)
	}

	outputMsg := dynamicpb.NewMessage(outputDesc)
	if err := conn.Invoke(ctx, method.fullMethod, inputMsg, outputMsg); err != nil {
		return nil, fmt.Errorf("call %s: %w", method.fullMethod, err)
	}

	outputBytes, err := protojson.MarshalOptions{}.Marshal(outputMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal output: %w", err)
	}

	return newCallResult(method.serviceName, method.methodDoc.GetName(), outputBytes), nil
}

func listMethodsViaDescribe(ctx context.Context, conn *grpc.ClientConn) ([]string, error) {
	catalog, err := getDescribeCatalog(ctx, conn)
	if err != nil {
		return nil, err
	}
	return catalog.availableMethods(), nil
}

func shouldFallbackToReflection(err error) bool {
	return errors.Is(err, errDescribeUnavailable) || errors.Is(err, errDescribeInsufficient)
}

func getDescribeCatalog(ctx context.Context, conn *grpc.ClientConn) (*describeCatalog, error) {
	if conn == nil {
		return nil, errors.New("gRPC connection is required")
	}

	entryAny, _ := describeCatalogCache.LoadOrStore(conn, &describeCacheEntry{})
	entry := entryAny.(*describeCacheEntry)

	entry.mu.Lock()
	if entry.ready {
		catalog, err := entry.catalog, entry.err
		entry.mu.Unlock()
		return catalog, err
	}
	entry.mu.Unlock()

	catalog, err := fetchDescribeCatalog(ctx, conn)
	if err != nil {
		if shouldFallbackToReflection(err) {
			entry.mu.Lock()
			if !entry.ready {
				entry.err = err
				entry.ready = true
			}
			err = entry.err
			entry.mu.Unlock()
		}
		return nil, err
	}

	entry.mu.Lock()
	if !entry.ready {
		entry.catalog = catalog
		entry.ready = true
	}
	catalog = entry.catalog
	entry.mu.Unlock()
	return catalog, nil
}

func fetchDescribeCatalog(ctx context.Context, conn *grpc.ClientConn) (*describeCatalog, error) {
	client := holonsv1.NewHolonMetaClient(conn)
	response, err := client.Describe(ctx, &holonsv1.DescribeRequest{})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if describeUnavailable(err) {
			return nil, fmt.Errorf("%w: %v", errDescribeUnavailable, err)
		}
		return nil, fmt.Errorf("HolonMeta.Describe: %w", err)
	}

	catalog, err := buildDescribeCatalog(response)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errDescribeInsufficient, err)
	}
	return catalog, nil
}

func describeUnavailable(err error) bool {
	code := status.Code(err)
	if code == codes.Unimplemented || code == codes.NotFound {
		return true
	}
	if code != codes.Unknown {
		return false
	}

	lowered := strings.ToLower(err.Error())
	return strings.Contains(lowered, "unknown service") ||
		strings.Contains(lowered, "unimplemented")
}

func buildDescribeCatalog(response *holonsv1.DescribeResponse) (*describeCatalog, error) {
	catalog := &describeCatalog{
		methods: make([]*describeMethod, 0),
		byExact: make(map[string]*describeMethod),
		byName:  make(map[string][]*describeMethod),
	}

	for _, service := range response.GetServices() {
		serviceName := strings.TrimSpace(service.GetName())
		if serviceName == "" {
			continue
		}

		for _, method := range service.GetMethods() {
			methodName := strings.TrimSpace(method.GetName())
			if methodName == "" {
				continue
			}

			fullMethod := "/" + serviceName + "/" + methodName
			binding := &describeMethod{
				serviceName: serviceName,
				methodDoc:   method,
				fullMethod:  fullMethod,
			}

			catalog.methods = append(catalog.methods, binding)
			catalog.byExact[strings.TrimPrefix(fullMethod, "/")] = binding
			catalog.byExact[fullMethod] = binding
			catalog.byName[methodName] = append(catalog.byName[methodName], binding)
		}
	}

	if len(catalog.methods) == 0 {
		return nil, errors.New("Describe returned no service methods")
	}

	sort.Slice(catalog.methods, func(i, j int) bool {
		return catalog.methods[i].fullMethod < catalog.methods[j].fullMethod
	})
	return catalog, nil
}

func (c *describeCatalog) resolve(methodName string) (*describeMethod, error) {
	if c == nil {
		return nil, errors.New("Describe catalog is not available")
	}

	trimmed := strings.TrimSpace(methodName)
	if trimmed == "" {
		return nil, errors.New("method name is required")
	}

	if strings.Contains(trimmed, "/") {
		if method, ok := c.byExact[strings.TrimPrefix(trimmed, "/")]; ok {
			return method, nil
		}
		return nil, fmt.Errorf("method %q not found. Available: %v", methodName, c.availableMethods())
	}

	name := canonicalMethodName(trimmed)
	matches := c.byName[name]
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("method %q not found. Available: %v", methodName, c.availableMethods())
	case 1:
		return matches[0], nil
	default:
		candidates := make([]string, 0, len(matches))
		for _, match := range matches {
			candidates = append(candidates, strings.TrimPrefix(match.fullMethod, "/"))
		}
		sort.Strings(candidates)
		return nil, fmt.Errorf("method %q is ambiguous. Use one of: %v", methodName, candidates)
	}
}

func (c *describeCatalog) availableMethods() []string {
	if c == nil {
		return nil
	}

	methods := make([]string, 0, len(c.methods))
	for _, method := range c.methods {
		methods = append(methods, strings.TrimPrefix(method.fullMethod, "/"))
	}
	return methods
}

func (m *describeMethod) descriptors() (protoreflect.MessageDescriptor, protoreflect.MessageDescriptor, error) {
	m.mu.Lock()
	if m.ready {
		inputDesc, outputDesc, err := m.inputDesc, m.outputDesc, m.buildErr
		m.mu.Unlock()
		return inputDesc, outputDesc, err
	}
	m.mu.Unlock()

	inputDesc, outputDesc, err := buildDescribeDescriptors(m.methodDoc)

	m.mu.Lock()
	if !m.ready {
		m.inputDesc = inputDesc
		m.outputDesc = outputDesc
		m.buildErr = err
		m.ready = true
	}
	inputDesc, outputDesc, err = m.inputDesc, m.outputDesc, m.buildErr
	m.mu.Unlock()
	return inputDesc, outputDesc, err
}

func buildDescribeDescriptors(method *holonsv1.MethodDoc) (protoreflect.MessageDescriptor, protoreflect.MessageDescriptor, error) {
	builder := newSyntheticProtoBuilder()

	inputDesc, inputSynthetic, err := builder.resolveRootMessage(method.GetInputType(), method.GetInputFields(), "Input")
	if err != nil {
		return nil, nil, err
	}
	outputDesc, outputSynthetic, err := builder.resolveRootMessage(method.GetOutputType(), method.GetOutputFields(), "Output")
	if err != nil {
		return nil, nil, err
	}

	if len(builder.file.GetMessageType()) == 0 && len(builder.file.GetEnumType()) == 0 {
		return inputDesc, outputDesc, nil
	}

	builder.finalizeDependencies()

	files := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{builder.file},
	}

	externalNames := make([]string, 0, len(builder.externalFiles))
	for name := range builder.externalFiles {
		externalNames = append(externalNames, name)
	}
	sort.Strings(externalNames)
	for _, name := range externalNames {
		files.File = append(files.File, builder.externalFiles[name])
	}

	fileDescs, err := protodesc.NewFiles(files)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: build synthetic descriptors: %v", errDescribeInsufficient, err)
	}

	if inputSynthetic != "" {
		inputDesc, err = findMessageDescriptor(fileDescs, inputSynthetic)
		if err != nil {
			return nil, nil, err
		}
	}
	if outputSynthetic != "" {
		outputDesc, err = findMessageDescriptor(fileDescs, outputSynthetic)
		if err != nil {
			return nil, nil, err
		}
	}

	return inputDesc, outputDesc, nil
}

func findMessageDescriptor(files *protoregistry.Files, fullName string) (protoreflect.MessageDescriptor, error) {
	if files == nil {
		return nil, errors.New("descriptor files are required")
	}

	descriptor, err := files.FindDescriptorByName(protoreflect.FullName(strings.TrimPrefix(fullName, ".")))
	if err != nil {
		return nil, fmt.Errorf("%w: find message %s: %v", errDescribeInsufficient, fullName, err)
	}

	message, ok := descriptor.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("%w: %s is not a message", errDescribeInsufficient, fullName)
	}
	return message, nil
}

type syntheticProtoBuilder struct {
	packageName string
	file        *descriptorpb.FileDescriptorProto

	externalFiles map[string]*descriptorpb.FileDescriptorProto
	directDeps    map[string]struct{}
	messageNames  map[string]string
	messageDefs   map[string]*descriptorpb.DescriptorProto
	enumNames     map[string]string
	enumDefs      map[string]*descriptorpb.EnumDescriptorProto

	nextMessageID int
	nextEnumID    int
	nextMapID     int
}

func newSyntheticProtoBuilder() *syntheticProtoBuilder {
	return &syntheticProtoBuilder{
		packageName:   "holons.dynamic",
		externalFiles: make(map[string]*descriptorpb.FileDescriptorProto),
		directDeps:    make(map[string]struct{}),
		messageNames:  make(map[string]string),
		messageDefs:   make(map[string]*descriptorpb.DescriptorProto),
		enumNames:     make(map[string]string),
		enumDefs:      make(map[string]*descriptorpb.EnumDescriptorProto),
		file: &descriptorpb.FileDescriptorProto{
			Name:    proto.String("holons/dynamic.proto"),
			Package: proto.String("holons.dynamic"),
			Syntax:  proto.String("proto3"),
		},
	}
}

func (b *syntheticProtoBuilder) resolveRootMessage(
	typeName string,
	fields []*holonsv1.FieldDoc,
	localName string,
) (protoreflect.MessageDescriptor, string, error) {
	if desc, ok, err := b.lookupExternalMessage(typeName); err != nil {
		return nil, "", err
	} else if ok && len(fields) == 0 {
		return desc, "", nil
	}

	fullName, err := b.ensureMessageType(typeName, fields, localName)
	if err != nil {
		return nil, "", err
	}
	return nil, fullName, nil
}

func (b *syntheticProtoBuilder) ensureMessageType(typeName string, fields []*holonsv1.FieldDoc, localName string) (string, error) {
	key := messageKey(typeName, localName)
	if fullName, ok := b.messageNames[key]; ok {
		msg := b.messageDefs[key]
		if len(msg.GetField()) == 0 && len(fields) > 0 {
			if err := b.populateMessageFields(msg, fields); err != nil {
				return "", err
			}
		}
		return fullName, nil
	}

	if localName == "" {
		b.nextMessageID++
		localName = fmt.Sprintf("Message%d", b.nextMessageID)
	}

	msg := &descriptorpb.DescriptorProto{Name: proto.String(localName)}
	fullName := "." + b.packageName + "." + localName
	b.messageNames[key] = fullName
	b.messageDefs[key] = msg
	b.file.MessageType = append(b.file.MessageType, msg)

	if err := b.populateMessageFields(msg, fields); err != nil {
		return "", err
	}
	return fullName, nil
}

func (b *syntheticProtoBuilder) populateMessageFields(msg *descriptorpb.DescriptorProto, fields []*holonsv1.FieldDoc) error {
	if msg == nil {
		return errors.New("message descriptor is required")
	}
	if len(fields) == 0 {
		return nil
	}

	descriptors := make([]*descriptorpb.FieldDescriptorProto, 0, len(fields))
	for _, field := range fields {
		descriptor, err := b.buildFieldDescriptor(field)
		if err != nil {
			return err
		}
		descriptors = append(descriptors, descriptor)
	}
	msg.Field = descriptors
	return nil
}

func (b *syntheticProtoBuilder) buildFieldDescriptor(field *holonsv1.FieldDoc) (*descriptorpb.FieldDescriptorProto, error) {
	if field == nil {
		return nil, errors.New("field descriptor is required")
	}

	fd := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(field.GetName()),
		Number: proto.Int32(field.GetNumber()),
	}

	if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_MAP {
		typeName, err := b.buildMapEntryDescriptor(field)
		if err != nil {
			return nil, err
		}
		fd.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		fd.TypeName = proto.String(typeName)
		return fd, nil
	}

	if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_REPEATED {
		fd.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	} else {
		fd.Label = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}

	switch {
	case len(field.GetEnumValues()) > 0:
		typeName, err := b.ensureEnumType(field.GetType(), field.GetEnumValues())
		if err != nil {
			return nil, err
		}
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		fd.TypeName = proto.String(typeName)
	case len(field.GetNestedFields()) > 0:
		typeName, err := b.ensureMessageType(field.GetType(), field.GetNestedFields(), "")
		if err != nil {
			return nil, err
		}
		fd.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		fd.TypeName = proto.String(typeName)
	default:
		if scalarType, ok := scalarFieldType(field.GetType()); ok {
			fd.Type = scalarType.Enum()
			return fd, nil
		}

		if typeName, kind, ok, err := b.lookupExternalType(field.GetType()); err != nil {
			return nil, err
		} else if ok {
			fd.Type = kind.Enum()
			fd.TypeName = proto.String(typeName)
			return fd, nil
		}

		if typeName, ok := b.enumNames[typeKey(field.GetType())]; ok {
			fd.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
			fd.TypeName = proto.String(typeName)
			return fd, nil
		}
		if typeName, ok := b.messageNames[messageKey(field.GetType(), "")]; ok {
			fd.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fd.TypeName = proto.String(typeName)
			return fd, nil
		}

		return nil, fmt.Errorf("%w: field %q type %q is not fully described", errDescribeInsufficient, field.GetName(), field.GetType())
	}

	return fd, nil
}

func (b *syntheticProtoBuilder) buildMapEntryDescriptor(field *holonsv1.FieldDoc) (string, error) {
	b.nextMapID++
	localName := fmt.Sprintf("MapEntry%d", b.nextMapID)
	fullName := "." + b.packageName + "." + localName

	entry := &descriptorpb.DescriptorProto{
		Name: proto.String(localName),
		Options: &descriptorpb.MessageOptions{
			MapEntry: proto.Bool(true),
		},
	}

	keyType, ok := scalarFieldType(field.GetMapKeyType())
	if !ok {
		return "", fmt.Errorf("%w: map field %q has unsupported key type %q", errDescribeInsufficient, field.GetName(), field.GetMapKeyType())
	}

	keyField := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String("key"),
		Number: proto.Int32(1),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   keyType.Enum(),
	}

	valueField := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String("value"),
		Number: proto.Int32(2),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
	}

	switch {
	case len(field.GetEnumValues()) > 0:
		typeName, err := b.ensureEnumType(field.GetMapValueType(), field.GetEnumValues())
		if err != nil {
			return "", err
		}
		valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
		valueField.TypeName = proto.String(typeName)
	case len(field.GetNestedFields()) > 0:
		typeName, err := b.ensureMessageType(field.GetMapValueType(), field.GetNestedFields(), "")
		if err != nil {
			return "", err
		}
		valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		valueField.TypeName = proto.String(typeName)
	default:
		if scalarType, ok := scalarFieldType(field.GetMapValueType()); ok {
			valueField.Type = scalarType.Enum()
		} else if typeName, kind, ok, err := b.lookupExternalType(field.GetMapValueType()); err != nil {
			return "", err
		} else if ok {
			valueField.Type = kind.Enum()
			valueField.TypeName = proto.String(typeName)
		} else if typeName, ok := b.enumNames[typeKey(field.GetMapValueType())]; ok {
			valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum()
			valueField.TypeName = proto.String(typeName)
		} else if typeName, ok := b.messageNames[messageKey(field.GetMapValueType(), "")]; ok {
			valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			valueField.TypeName = proto.String(typeName)
		} else {
			return "", fmt.Errorf("%w: map field %q value type %q is not fully described", errDescribeInsufficient, field.GetName(), field.GetMapValueType())
		}
	}

	entry.Field = []*descriptorpb.FieldDescriptorProto{keyField, valueField}
	b.file.MessageType = append(b.file.MessageType, entry)
	return fullName, nil
}

func (b *syntheticProtoBuilder) ensureEnumType(typeName string, values []*holonsv1.EnumValueDoc) (string, error) {
	key := typeKey(typeName)
	if fullName, ok := b.enumNames[key]; ok {
		enum := b.enumDefs[key]
		if len(enum.GetValue()) == 0 && len(values) > 0 {
			enum.Value = buildEnumValueDescriptors(values)
		}
		return fullName, nil
	}

	if len(values) == 0 {
		return "", fmt.Errorf("%w: enum type %q has no values", errDescribeInsufficient, typeName)
	}

	b.nextEnumID++
	localName := fmt.Sprintf("Enum%d", b.nextEnumID)
	enum := &descriptorpb.EnumDescriptorProto{
		Name:  proto.String(localName),
		Value: buildEnumValueDescriptors(values),
	}
	fullName := "." + b.packageName + "." + localName

	b.enumNames[key] = fullName
	b.enumDefs[key] = enum
	b.file.EnumType = append(b.file.EnumType, enum)
	return fullName, nil
}

func buildEnumValueDescriptors(values []*holonsv1.EnumValueDoc) []*descriptorpb.EnumValueDescriptorProto {
	descriptors := make([]*descriptorpb.EnumValueDescriptorProto, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		descriptors = append(descriptors, &descriptorpb.EnumValueDescriptorProto{
			Name:   proto.String(value.GetName()),
			Number: proto.Int32(value.GetNumber()),
		})
	}
	return descriptors
}

func (b *syntheticProtoBuilder) lookupExternalMessage(typeName string) (protoreflect.MessageDescriptor, bool, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(typeName), ".")
	if trimmed == "" {
		return nil, false, nil
	}

	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(trimmed))
	if err != nil {
		return nil, false, nil
	}

	message, ok := descriptor.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, false, nil
	}
	if err := b.addExternalFile(message.ParentFile()); err != nil {
		return nil, false, err
	}
	b.directDeps[message.ParentFile().Path()] = struct{}{}
	return message, true, nil
}

func (b *syntheticProtoBuilder) lookupExternalType(typeName string) (string, descriptorpb.FieldDescriptorProto_Type, bool, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(typeName), ".")
	if trimmed == "" {
		return "", 0, false, nil
	}

	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(trimmed))
	if err != nil {
		return "", 0, false, nil
	}

	if err := b.addExternalFile(descriptor.ParentFile()); err != nil {
		return "", 0, false, err
	}
	b.directDeps[descriptor.ParentFile().Path()] = struct{}{}

	switch descriptor.(type) {
	case protoreflect.MessageDescriptor:
		return "." + trimmed, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, true, nil
	case protoreflect.EnumDescriptor:
		return "." + trimmed, descriptorpb.FieldDescriptorProto_TYPE_ENUM, true, nil
	default:
		return "", 0, false, nil
	}
}

func (b *syntheticProtoBuilder) addExternalFile(file protoreflect.FileDescriptor) error {
	if file == nil {
		return nil
	}

	name := file.Path()
	if _, exists := b.externalFiles[name]; exists {
		return nil
	}

	imports := file.Imports()
	for i := 0; i < imports.Len(); i++ {
		if err := b.addExternalFile(imports.Get(i).FileDescriptor); err != nil {
			return err
		}
	}

	b.externalFiles[name] = protodesc.ToFileDescriptorProto(file)
	return nil
}

func (b *syntheticProtoBuilder) finalizeDependencies() {
	if len(b.directDeps) == 0 {
		return
	}

	deps := make([]string, 0, len(b.directDeps))
	for dep := range b.directDeps {
		deps = append(deps, dep)
	}
	sort.Strings(deps)
	b.file.Dependency = deps
}

func typeKey(name string) string {
	return strings.TrimPrefix(strings.TrimSpace(name), ".")
}

func messageKey(typeName string, localName string) string {
	if key := typeKey(typeName); key != "" {
		return key
	}
	return "#message:" + localName
}

func scalarFieldType(typeName string) (descriptorpb.FieldDescriptorProto_Type, bool) {
	switch strings.TrimSpace(typeName) {
	case "double":
		return descriptorpb.FieldDescriptorProto_TYPE_DOUBLE, true
	case "float":
		return descriptorpb.FieldDescriptorProto_TYPE_FLOAT, true
	case "int64":
		return descriptorpb.FieldDescriptorProto_TYPE_INT64, true
	case "uint64":
		return descriptorpb.FieldDescriptorProto_TYPE_UINT64, true
	case "int32":
		return descriptorpb.FieldDescriptorProto_TYPE_INT32, true
	case "fixed64":
		return descriptorpb.FieldDescriptorProto_TYPE_FIXED64, true
	case "fixed32":
		return descriptorpb.FieldDescriptorProto_TYPE_FIXED32, true
	case "bool":
		return descriptorpb.FieldDescriptorProto_TYPE_BOOL, true
	case "string":
		return descriptorpb.FieldDescriptorProto_TYPE_STRING, true
	case "group":
		return descriptorpb.FieldDescriptorProto_TYPE_GROUP, true
	case "bytes":
		return descriptorpb.FieldDescriptorProto_TYPE_BYTES, true
	case "uint32":
		return descriptorpb.FieldDescriptorProto_TYPE_UINT32, true
	case "sfixed32":
		return descriptorpb.FieldDescriptorProto_TYPE_SFIXED32, true
	case "sfixed64":
		return descriptorpb.FieldDescriptorProto_TYPE_SFIXED64, true
	case "sint32":
		return descriptorpb.FieldDescriptorProto_TYPE_SINT32, true
	case "sint64":
		return descriptorpb.FieldDescriptorProto_TYPE_SINT64, true
	default:
		return 0, false
	}
}

func newCallResult(serviceName string, methodName string, outputBytes []byte) *CallResult {
	var pretty json.RawMessage
	if err := json.Unmarshal(outputBytes, &pretty); err != nil {
		return &CallResult{
			Service: serviceName,
			Method:  methodName,
			Output:  string(outputBytes),
		}
	}

	prettyBytes, _ := json.MarshalIndent(pretty, "", "  ")
	return &CallResult{
		Service: serviceName,
		Method:  methodName,
		Output:  string(prettyBytes),
	}
}
