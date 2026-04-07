package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	holonsv1 "github.com/organic-programming/go-holons/gen/go/holons/v1"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type messageSchema struct {
	fullName string
	parent   string
	fields   []*holonsv1.FieldDoc
}

type enumSchema struct {
	fullName string
	parent   string
	values   []*holonsv1.EnumValueDoc
}

type staticSchemaBuilder struct {
	messages          map[string]*messageSchema
	enums             map[string]*enumSchema
	servicesByPackage map[string][]*holonsv1.ServiceDoc
}

func loadMethodRegistryFromDescribeResponse(response *holonsv1.DescribeResponse) (map[string]protoreflect.MethodDescriptor, error) {
	builder, err := newStaticSchemaBuilder(response)
	if err != nil {
		return nil, err
	}

	descriptorSet, err := builder.buildFileDescriptorSet()
	if err != nil {
		return nil, err
	}

	registry, err := protodesc.NewFiles(descriptorSet)
	if err != nil {
		return nil, fmt.Errorf("build proto registry from static describe: %w", err)
	}

	methods := map[string]protoreflect.MethodDescriptor{}
	registry.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			service := services.Get(i)
			if string(service.FullName()) == "holons.v1.HolonMeta" {
				continue
			}
			serviceMethods := service.Methods()
			for j := 0; j < serviceMethods.Len(); j++ {
				method := serviceMethods.Get(j)
				fullMethod := "/" + string(service.FullName()) + "/" + string(method.Name())
				methods[fullMethod] = method
			}
		}
		return true
	})
	return methods, nil
}

func newStaticSchemaBuilder(response *holonsv1.DescribeResponse) (*staticSchemaBuilder, error) {
	builder := &staticSchemaBuilder{
		messages:          map[string]*messageSchema{},
		enums:             map[string]*enumSchema{},
		servicesByPackage: map[string][]*holonsv1.ServiceDoc{},
	}

	for _, service := range response.GetServices() {
		if service == nil {
			continue
		}
		serviceName := normalizeFullName(service.GetName())
		if serviceName == "" {
			return nil, fmt.Errorf("static describe contains a service without a name")
		}
		packageName, _ := splitQualifiedName(serviceName)
		if packageName == "" {
			return nil, fmt.Errorf("service %q is missing a package", serviceName)
		}
		builder.servicesByPackage[packageName] = append(builder.servicesByPackage[packageName], service)

		for _, method := range service.GetMethods() {
			if method == nil {
				continue
			}
			if err := builder.registerMessage(normalizeFullName(method.GetInputType()), "", method.GetInputFields()); err != nil {
				return nil, fmt.Errorf("%s/%s input schema: %w", serviceName, method.GetName(), err)
			}
			if err := builder.registerMessage(normalizeFullName(method.GetOutputType()), "", method.GetOutputFields()); err != nil {
				return nil, fmt.Errorf("%s/%s output schema: %w", serviceName, method.GetName(), err)
			}
		}
	}

	return builder, nil
}

func (b *staticSchemaBuilder) registerMessage(fullName string, parent string, fields []*holonsv1.FieldDoc) error {
	fullName = normalizeFullName(fullName)
	parent = normalizeFullName(parent)
	if fullName == "" {
		return fmt.Errorf("message type is required")
	}
	if parent != "" && !strings.HasPrefix(fullName, parent+".") {
		parent = ""
	}

	if existing, ok := b.messages[fullName]; ok {
		if existing.parent == "" && parent != "" {
			existing.parent = parent
		}
		if len(existing.fields) == 0 && len(fields) > 0 {
			existing.fields = fields
		}
	} else {
		b.messages[fullName] = &messageSchema{
			fullName: fullName,
			parent:   parent,
			fields:   fields,
		}
	}

	for _, field := range fields {
		if err := b.registerNestedTypes(fullName, field); err != nil {
			return err
		}
	}
	return nil
}

func (b *staticSchemaBuilder) registerNestedTypes(parent string, field *holonsv1.FieldDoc) error {
	if field == nil {
		return nil
	}

	childParent := func(typeName string) string {
		typeName = normalizeFullName(typeName)
		if strings.HasPrefix(typeName, parent+".") {
			return parent
		}
		return ""
	}

	if nested := field.GetNestedFields(); len(nested) > 0 {
		typeName := normalizeFullName(field.GetType())
		if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_MAP {
			typeName = normalizeFullName(field.GetMapValueType())
		}
		if typeName != "" {
			if err := b.registerMessage(typeName, childParent(typeName), nested); err != nil {
				return err
			}
		}
	}

	if values := field.GetEnumValues(); len(values) > 0 {
		typeName := normalizeFullName(field.GetType())
		if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_MAP {
			typeName = normalizeFullName(field.GetMapValueType())
		}
		if typeName != "" {
			if err := b.registerEnum(typeName, childParent(typeName), values); err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *staticSchemaBuilder) registerEnum(fullName string, parent string, values []*holonsv1.EnumValueDoc) error {
	fullName = normalizeFullName(fullName)
	parent = normalizeFullName(parent)
	if fullName == "" {
		return fmt.Errorf("enum type is required")
	}
	if parent != "" && !strings.HasPrefix(fullName, parent+".") {
		parent = ""
	}

	if existing, ok := b.enums[fullName]; ok {
		if existing.parent == "" && parent != "" {
			existing.parent = parent
		}
		if len(existing.values) == 0 && len(values) > 0 {
			existing.values = values
		}
		return nil
	}

	b.enums[fullName] = &enumSchema{
		fullName: fullName,
		parent:   parent,
		values:   values,
	}
	return nil
}

func (b *staticSchemaBuilder) buildFileDescriptorSet() (*descriptorpb.FileDescriptorSet, error) {
	packages := map[string]struct{}{}
	for packageName := range b.servicesByPackage {
		packages[packageName] = struct{}{}
	}
	for _, message := range b.messages {
		if message == nil || message.parent != "" {
			continue
		}
		packages[b.packageForName(message.fullName)] = struct{}{}
	}
	for _, enum := range b.enums {
		if enum == nil || enum.parent != "" {
			continue
		}
		packages[b.packageForName(enum.fullName)] = struct{}{}
	}

	packageNames := make([]string, 0, len(packages))
	for packageName := range packages {
		if strings.TrimSpace(packageName) == "" {
			continue
		}
		packageNames = append(packageNames, packageName)
	}
	sort.Strings(packageNames)

	set := &descriptorpb.FileDescriptorSet{}
	for _, packageName := range packageNames {
		file := &descriptorpb.FileDescriptorProto{
			Name:    proto.String(strings.ReplaceAll(packageName, ".", "/") + "/describe_static.proto"),
			Package: proto.String(packageName),
			Syntax:  proto.String("proto3"),
		}

		for _, dependency := range b.dependenciesForPackage(packageName) {
			file.Dependency = append(file.Dependency, strings.ReplaceAll(dependency, ".", "/")+"/describe_static.proto")
		}

		for _, fullName := range b.topLevelMessageNames(packageName) {
			message, err := b.buildMessageDescriptor(fullName)
			if err != nil {
				return nil, err
			}
			file.MessageType = append(file.MessageType, message)
		}

		for _, fullName := range b.topLevelEnumNames(packageName) {
			file.EnumType = append(file.EnumType, b.buildEnumDescriptor(fullName))
		}

		services := append([]*holonsv1.ServiceDoc(nil), b.servicesByPackage[packageName]...)
		sort.Slice(services, func(i, j int) bool {
			return services[i].GetName() < services[j].GetName()
		})
		for _, service := range services {
			serviceDescriptor, err := b.buildServiceDescriptor(service)
			if err != nil {
				return nil, err
			}
			file.Service = append(file.Service, serviceDescriptor)
		}

		set.File = append(set.File, file)
	}

	return set, nil
}

func (b *staticSchemaBuilder) buildServiceDescriptor(service *holonsv1.ServiceDoc) (*descriptorpb.ServiceDescriptorProto, error) {
	serviceName := normalizeFullName(service.GetName())
	_, localName := splitQualifiedName(serviceName)
	if localName == "" {
		return nil, fmt.Errorf("service %q is missing a local name", serviceName)
	}

	out := &descriptorpb.ServiceDescriptorProto{
		Name: proto.String(localName),
	}

	methods := append([]*holonsv1.MethodDoc(nil), service.GetMethods()...)
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].GetName() < methods[j].GetName()
	})

	for _, method := range methods {
		if method == nil {
			continue
		}
		inputType := normalizeFullName(method.GetInputType())
		outputType := normalizeFullName(method.GetOutputType())
		if inputType == "" || outputType == "" {
			return nil, fmt.Errorf("%s/%s is missing an input or output type", serviceName, method.GetName())
		}
		out.Method = append(out.Method, &descriptorpb.MethodDescriptorProto{
			Name:            proto.String(method.GetName()),
			InputType:       proto.String("." + inputType),
			OutputType:      proto.String("." + outputType),
			ClientStreaming: proto.Bool(method.GetClientStreaming()),
			ServerStreaming: proto.Bool(method.GetServerStreaming()),
		})
	}
	return out, nil
}

func (b *staticSchemaBuilder) buildMessageDescriptor(fullName string) (*descriptorpb.DescriptorProto, error) {
	message, ok := b.messages[fullName]
	if !ok || message == nil {
		return nil, fmt.Errorf("message schema %q is missing", fullName)
	}

	localName, err := b.localName(fullName, message.parent)
	if err != nil {
		return nil, err
	}

	out := &descriptorpb.DescriptorProto{
		Name: proto.String(localName),
	}

	fields := append([]*holonsv1.FieldDoc(nil), message.fields...)
	sort.Slice(fields, func(i, j int) bool {
		return fields[i].GetNumber() < fields[j].GetNumber()
	})

	for _, field := range fields {
		fieldDescriptor, mapEntry, err := b.buildFieldDescriptor(fullName, field)
		if err != nil {
			return nil, err
		}
		out.Field = append(out.Field, fieldDescriptor)
		if mapEntry != nil {
			out.NestedType = append(out.NestedType, mapEntry)
		}
	}

	for _, childName := range b.childMessageNames(fullName) {
		childDescriptor, err := b.buildMessageDescriptor(childName)
		if err != nil {
			return nil, err
		}
		out.NestedType = append(out.NestedType, childDescriptor)
	}

	for _, childName := range b.childEnumNames(fullName) {
		out.EnumType = append(out.EnumType, b.buildEnumDescriptor(childName))
	}

	return out, nil
}

func (b *staticSchemaBuilder) buildFieldDescriptor(parentFullName string, field *holonsv1.FieldDoc) (*descriptorpb.FieldDescriptorProto, *descriptorpb.DescriptorProto, error) {
	if field == nil {
		return nil, nil, fmt.Errorf("field schema for %s is nil", parentFullName)
	}
	if strings.TrimSpace(field.GetName()) == "" {
		return nil, nil, fmt.Errorf("message %s contains a field without a name", parentFullName)
	}

	out := &descriptorpb.FieldDescriptorProto{
		Name:     proto.String(field.GetName()),
		JsonName: proto.String(lowerCamelCase(field.GetName())),
		Number:   proto.Int32(field.GetNumber()),
	}

	if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_MAP {
		mapEntry, err := b.buildMapEntryDescriptor(parentFullName, field)
		if err != nil {
			return nil, nil, err
		}
		out.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		out.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
		out.TypeName = proto.String("." + parentFullName + "." + mapEntry.GetName())
		return out, mapEntry, nil
	}

	if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_REPEATED {
		out.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	} else {
		out.Label = descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum()
	}

	fieldType, typeName, err := b.resolveNamedOrScalarType(field.GetType(), field.GetNestedFields(), field.GetEnumValues())
	if err != nil {
		return nil, nil, fmt.Errorf("message %s field %s: %w", parentFullName, field.GetName(), err)
	}
	out.Type = fieldType.Enum()
	if typeName != "" {
		out.TypeName = proto.String("." + typeName)
	}
	return out, nil, nil
}

func (b *staticSchemaBuilder) buildMapEntryDescriptor(parentFullName string, field *holonsv1.FieldDoc) (*descriptorpb.DescriptorProto, error) {
	entryName := protoMapEntryName(field.GetName())

	keyType, keyTypeName, err := b.resolveNamedOrScalarType(field.GetMapKeyType(), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("message %s map field %s key: %w", parentFullName, field.GetName(), err)
	}
	if keyType == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE || keyType == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
		return nil, fmt.Errorf("message %s map field %s uses unsupported key type %q", parentFullName, field.GetName(), field.GetMapKeyType())
	}

	valueType, valueTypeName, err := b.resolveNamedOrScalarType(field.GetMapValueType(), field.GetNestedFields(), field.GetEnumValues())
	if err != nil {
		return nil, fmt.Errorf("message %s map field %s value: %w", parentFullName, field.GetName(), err)
	}

	entry := &descriptorpb.DescriptorProto{
		Name: proto.String(entryName),
		Options: &descriptorpb.MessageOptions{
			MapEntry: proto.Bool(true),
		},
		Field: []*descriptorpb.FieldDescriptorProto{
			{
				Name:     proto.String("key"),
				JsonName: proto.String("key"),
				Number:   proto.Int32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     keyType.Enum(),
			},
			{
				Name:     proto.String("value"),
				JsonName: proto.String("value"),
				Number:   proto.Int32(2),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     valueType.Enum(),
			},
		},
	}
	if keyTypeName != "" {
		entry.Field[0].TypeName = proto.String("." + keyTypeName)
	}
	if valueTypeName != "" {
		entry.Field[1].TypeName = proto.String("." + valueTypeName)
	}
	return entry, nil
}

func (b *staticSchemaBuilder) resolveNamedOrScalarType(typeName string, nestedFields []*holonsv1.FieldDoc, enumValues []*holonsv1.EnumValueDoc) (descriptorpb.FieldDescriptorProto_Type, string, error) {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, "", fmt.Errorf("type is required")
	}
	if scalar, ok := scalarFieldType(typeName); ok {
		return scalar, "", nil
	}

	fullName := normalizeFullName(typeName)
	if len(enumValues) > 0 {
		return descriptorpb.FieldDescriptorProto_TYPE_ENUM, fullName, nil
	}
	if len(nestedFields) > 0 {
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, fullName, nil
	}
	if _, ok := b.enums[fullName]; ok {
		return descriptorpb.FieldDescriptorProto_TYPE_ENUM, fullName, nil
	}
	if _, ok := b.messages[fullName]; ok {
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, fullName, nil
	}
	return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, fullName, nil
}

func (b *staticSchemaBuilder) buildEnumDescriptor(fullName string) *descriptorpb.EnumDescriptorProto {
	enumSchema := b.enums[fullName]
	localName, err := b.localName(fullName, enumSchema.parent)
	if err != nil {
		localName = fullName
	}

	out := &descriptorpb.EnumDescriptorProto{
		Name: proto.String(localName),
	}

	values := append([]*holonsv1.EnumValueDoc(nil), enumSchema.values...)
	sort.Slice(values, func(i, j int) bool {
		return values[i].GetNumber() < values[j].GetNumber()
	})
	for _, value := range values {
		if value == nil {
			continue
		}
		out.Value = append(out.Value, &descriptorpb.EnumValueDescriptorProto{
			Name:   proto.String(value.GetName()),
			Number: proto.Int32(value.GetNumber()),
		})
	}
	return out
}

func (b *staticSchemaBuilder) topLevelMessageNames(packageName string) []string {
	names := make([]string, 0)
	for fullName, message := range b.messages {
		if message == nil || message.parent != "" || b.packageForName(fullName) != packageName {
			continue
		}
		names = append(names, fullName)
	}
	sort.Strings(names)
	return names
}

func (b *staticSchemaBuilder) topLevelEnumNames(packageName string) []string {
	names := make([]string, 0)
	for fullName, enum := range b.enums {
		if enum == nil || enum.parent != "" || b.packageForName(fullName) != packageName {
			continue
		}
		names = append(names, fullName)
	}
	sort.Strings(names)
	return names
}

func (b *staticSchemaBuilder) childMessageNames(parent string) []string {
	names := make([]string, 0)
	for fullName, message := range b.messages {
		if message == nil || message.parent != parent {
			continue
		}
		names = append(names, fullName)
	}
	sort.Strings(names)
	return names
}

func (b *staticSchemaBuilder) childEnumNames(parent string) []string {
	names := make([]string, 0)
	for fullName, enum := range b.enums {
		if enum == nil || enum.parent != parent {
			continue
		}
		names = append(names, fullName)
	}
	sort.Strings(names)
	return names
}

func (b *staticSchemaBuilder) dependenciesForPackage(packageName string) []string {
	deps := map[string]struct{}{}
	for _, service := range b.servicesByPackage[packageName] {
		for _, method := range service.GetMethods() {
			for _, typeName := range []string{method.GetInputType(), method.GetOutputType()} {
				dependency := b.packageForName(normalizeFullName(typeName))
				if dependency != "" && dependency != packageName {
					deps[dependency] = struct{}{}
				}
			}
		}
	}
	for _, messageName := range b.topLevelMessageNames(packageName) {
		b.collectMessageDependencies(messageName, packageName, deps)
	}

	out := make([]string, 0, len(deps))
	for dependency := range deps {
		out = append(out, dependency)
	}
	sort.Strings(out)
	return out
}

func (b *staticSchemaBuilder) collectMessageDependencies(fullName string, packageName string, deps map[string]struct{}) {
	message := b.messages[fullName]
	if message == nil {
		return
	}
	for _, field := range message.fields {
		for _, reference := range referencedNamedTypes(field) {
			dependency := b.packageForName(reference)
			if dependency != "" && dependency != packageName {
				deps[dependency] = struct{}{}
			}
		}
	}
	for _, childName := range b.childMessageNames(fullName) {
		b.collectMessageDependencies(childName, packageName, deps)
	}
}

func referencedNamedTypes(field *holonsv1.FieldDoc) []string {
	if field == nil {
		return nil
	}

	references := make([]string, 0, 2)
	if field.GetLabel() == holonsv1.FieldLabel_FIELD_LABEL_MAP {
		if _, ok := scalarFieldType(field.GetMapValueType()); !ok {
			if name := normalizeFullName(field.GetMapValueType()); name != "" {
				references = append(references, name)
			}
		}
		return references
	}

	if _, ok := scalarFieldType(field.GetType()); ok {
		return references
	}
	if name := normalizeFullName(field.GetType()); name != "" {
		references = append(references, name)
	}
	return references
}

func (b *staticSchemaBuilder) packageForName(fullName string) string {
	fullName = normalizeFullName(fullName)
	if fullName == "" {
		return ""
	}

	if message, ok := b.messages[fullName]; ok && message != nil {
		if message.parent != "" {
			return b.packageForName(message.parent)
		}
		packageName, _ := splitQualifiedName(message.fullName)
		return packageName
	}
	if enum, ok := b.enums[fullName]; ok && enum != nil {
		if enum.parent != "" {
			return b.packageForName(enum.parent)
		}
		packageName, _ := splitQualifiedName(enum.fullName)
		return packageName
	}

	packageName, _ := splitQualifiedName(fullName)
	return packageName
}

func (b *staticSchemaBuilder) localName(fullName string, parent string) (string, error) {
	fullName = normalizeFullName(fullName)
	parent = normalizeFullName(parent)
	if fullName == "" {
		return "", fmt.Errorf("full name is required")
	}
	if parent == "" {
		_, localName := splitQualifiedName(fullName)
		if localName == "" {
			return "", fmt.Errorf("type %q is missing a local name", fullName)
		}
		return localName, nil
	}

	remainder := strings.TrimPrefix(fullName, parent+".")
	if remainder == fullName || remainder == "" || strings.Contains(remainder, ".") {
		return "", fmt.Errorf("type %q is not a direct child of %q", fullName, parent)
	}
	return remainder, nil
}

func splitQualifiedName(fullName string) (string, string) {
	trimmed := normalizeFullName(fullName)
	index := strings.LastIndex(trimmed, ".")
	if index < 0 {
		return "", trimmed
	}
	return trimmed[:index], trimmed[index+1:]
}

func normalizeFullName(fullName string) string {
	return strings.Trim(strings.TrimSpace(fullName), ".")
}

func scalarFieldType(typeName string) (descriptorpb.FieldDescriptorProto_Type, bool) {
	switch strings.ToLower(strings.TrimSpace(typeName)) {
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
		return descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, false
	}
}

func protoMapEntryName(fieldName string) string {
	name := camelCase(fieldName)
	if name == "" {
		name = "Field"
	}
	return name + "Entry"
}

func lowerCamelCase(name string) string {
	camel := camelCase(name)
	if camel == "" {
		return name
	}
	runes := []rune(camel)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func camelCase(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	if len(parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(strings.ToLower(part))
		runes[0] = unicode.ToUpper(runes[0])
		builder.WriteString(string(runes))
	}
	return builder.String()
}
