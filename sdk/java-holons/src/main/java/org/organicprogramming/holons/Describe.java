package org.organicprogramming.holons;

import io.grpc.MethodDescriptor;
import io.grpc.ServerServiceDefinition;
import io.grpc.protobuf.ProtoUtils;
import io.grpc.stub.ServerCalls;
import holons.v1.Manifest;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.Deque;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Stream;

/** Build-time describe helpers plus runtime registration for a static HolonMeta response. */
public final class Describe {
    private static final String HOLON_META_SERVICE = "holons.v1.HolonMeta";
    private static final Pattern PACKAGE_PATTERN = Pattern.compile("^package\\s+([A-Za-z0-9_.]+)\\s*;");
    private static final Pattern SERVICE_PATTERN = Pattern.compile("^service\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*\\{?");
    private static final Pattern MESSAGE_PATTERN = Pattern.compile("^message\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*\\{?");
    private static final Pattern ENUM_PATTERN = Pattern.compile("^enum\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*\\{?");
    private static final Pattern RPC_PATTERN = Pattern.compile(
            "^rpc\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*\\(\\s*(stream\\s+)?([.A-Za-z0-9_]+)\\s*\\)\\s*returns\\s*\\(\\s*(stream\\s+)?([.A-Za-z0-9_]+)\\s*\\)");
    private static final Pattern MAP_FIELD_PATTERN = Pattern.compile(
            "^(repeated\\s+)?map\\s*<\\s*([.A-Za-z0-9_]+)\\s*,\\s*([.A-Za-z0-9_]+)\\s*>\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*=\\s*(\\d+)\\s*;");
    private static final Pattern FIELD_PATTERN = Pattern.compile(
            "^(optional\\s+|repeated\\s+)?([.A-Za-z0-9_]+)\\s+([A-Za-z_][A-Za-z0-9_]*)\\s*=\\s*(\\d+)\\s*;");
    private static final Pattern ENUM_VALUE_PATTERN = Pattern.compile(
            "^([A-Za-z_][A-Za-z0-9_]*)\\s*=\\s*(-?\\d+)\\s*;");

    private static final Set<String> SCALARS = Set.of(
            "double", "float", "int64", "uint64", "int32", "fixed64", "fixed32",
            "bool", "string", "bytes", "uint32", "sfixed32", "sfixed64",
            "sint32", "sint64");

    private static final MethodDescriptor<holons.v1.Describe.DescribeRequest, holons.v1.Describe.DescribeResponse> DESCRIBE_METHOD =
            MethodDescriptor.<holons.v1.Describe.DescribeRequest, holons.v1.Describe.DescribeResponse>newBuilder()
                    .setType(MethodDescriptor.MethodType.UNARY)
                    .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_META_SERVICE, "Describe"))
                    .setRequestMarshaller(ProtoUtils.marshaller(holons.v1.Describe.DescribeRequest.getDefaultInstance()))
                    .setResponseMarshaller(ProtoUtils.marshaller(holons.v1.Describe.DescribeResponse.getDefaultInstance()))
                    .build();
    public static final String NO_INCODE_DESCRIPTION_MESSAGE = "no Incode Description registered — run op build";

    private static volatile holons.v1.Describe.DescribeResponse staticResponse;

    private Describe() {
    }

    public static MethodDescriptor<holons.v1.Describe.DescribeRequest, holons.v1.Describe.DescribeResponse> describeMethod() {
        return DESCRIBE_METHOD;
    }

    public static void useStaticResponse(holons.v1.Describe.DescribeResponse response) {
        staticResponse = cloneResponse(response);
    }

    public static ServerServiceDefinition service() {
        holons.v1.Describe.DescribeResponse response = registeredStaticResponse();
        return ServerServiceDefinition.builder(HOLON_META_SERVICE)
                .addMethod(
                        DESCRIBE_METHOD,
                        ServerCalls.asyncUnaryCall((request, observer) -> {
                            observer.onNext(response);
                            observer.onCompleted();
                        }))
                .build();
    }

    private static holons.v1.Describe.DescribeResponse registeredStaticResponse() {
        holons.v1.Describe.DescribeResponse response = staticResponse;
        if (response == null) {
            throw new IllegalStateException(NO_INCODE_DESCRIPTION_MESSAGE);
        }
        return cloneResponse(response);
    }

    private static holons.v1.Describe.DescribeResponse cloneResponse(
            holons.v1.Describe.DescribeResponse response) {
        return response == null ? null : response.toBuilder().build();
    }

    /** Build-time utility for op build and tests. Not used at runtime serve startup. */
    public static holons.v1.Describe.DescribeResponse buildResponse(Path protoDir) throws IOException {
        Objects.requireNonNull(protoDir, "protoDir");
        return buildResponse(protoDir, Identity.resolveManifestPath(protoDir));
    }

    /** Build-time utility for op build and tests. Not used at runtime serve startup. */
    public static holons.v1.Describe.DescribeResponse buildResponse(Path protoDir, Path manifestPath) throws IOException {
        Objects.requireNonNull(protoDir, "protoDir");
        Objects.requireNonNull(manifestPath, "manifestPath");

        Identity.ResolvedManifest resolved = Identity.resolveProtoFile(manifestPath);
        ProtoIndex index = parseProtoDirectory(protoDir);

        holons.v1.Describe.DescribeResponse.Builder response = holons.v1.Describe.DescribeResponse.newBuilder()
                .setManifest(protoManifest(resolved));

        for (ServiceDef service : index.services) {
            if (HOLON_META_SERVICE.equals(service.fullName)) {
                continue;
            }
            response.addServices(toServiceDoc(service, index));
        }

        return response.build();
    }

    private static Manifest.HolonManifest protoManifest(Identity.ResolvedManifest resolved) {
        Manifest.HolonManifest.Builder manifest = Manifest.HolonManifest.newBuilder()
                .setIdentity(Manifest.HolonManifest.Identity.newBuilder()
                        .setUuid(resolved.identity().uuid())
                        .setGivenName(resolved.identity().givenName())
                        .setFamilyName(resolved.identity().familyName())
                        .setMotto(resolved.identity().motto())
                        .setComposer(resolved.identity().composer())
                        .setStatus(resolved.identity().status())
                        .setBorn(resolved.identity().born())
                        .setVersion(resolved.identity().version())
                        .addAllAliases(resolved.identity().aliases())
                        .build())
                .setDescription(resolved.description())
                .setLang(resolved.identity().lang())
                .setKind(resolved.kind())
                .setBuild(Manifest.HolonManifest.Build.newBuilder()
                        .setRunner(resolved.buildRunner())
                        .setMain(resolved.buildMain())
                        .build())
                .setArtifacts(Manifest.HolonManifest.Artifacts.newBuilder()
                        .setBinary(resolved.artifactBinary())
                        .setPrimary(resolved.artifactPrimary())
                        .build());

        if (!resolved.requiredFiles().isEmpty()) {
            manifest.setRequires(Manifest.HolonManifest.Requires.newBuilder()
                    .addAllFiles(resolved.requiredFiles())
                    .build());
        }
        for (Identity.ResolvedSkill skill : resolved.skills()) {
            manifest.addSkills(Manifest.HolonManifest.Skill.newBuilder()
                    .setName(skill.name())
                    .setDescription(skill.description())
                    .setWhen(skill.when())
                    .addAllSteps(skill.steps())
                    .build());
        }
        for (Identity.ResolvedSequence sequence : resolved.sequences()) {
            Manifest.HolonManifest.Sequence.Builder sequenceBuilder = Manifest.HolonManifest.Sequence.newBuilder()
                    .setName(sequence.name())
                    .setDescription(sequence.description())
                    .addAllSteps(sequence.steps());
            for (Identity.ResolvedSequenceParam param : sequence.params()) {
                sequenceBuilder.addParams(Manifest.HolonManifest.Sequence.Param.newBuilder()
                        .setName(param.name())
                        .setDescription(param.description())
                        .setRequired(param.required())
                        .setDefault(param.defaultValue())
                        .build());
            }
            manifest.addSequences(sequenceBuilder.build());
        }

        return manifest.build();
    }

    private static holons.v1.Describe.ServiceDoc toServiceDoc(ServiceDef service, ProtoIndex index) {
        holons.v1.Describe.ServiceDoc.Builder builder = holons.v1.Describe.ServiceDoc.newBuilder()
                .setName(service.fullName)
                .setDescription(service.comment.description);
        for (MethodDef method : service.methods) {
            builder.addMethods(toMethodDoc(method, index));
        }
        return builder.build();
    }

    private static holons.v1.Describe.MethodDoc toMethodDoc(MethodDef method, ProtoIndex index) {
        holons.v1.Describe.MethodDoc.Builder builder = holons.v1.Describe.MethodDoc.newBuilder()
                .setName(method.name)
                .setDescription(method.comment.description)
                .setInputType(method.inputType)
                .setOutputType(method.outputType)
                .setClientStreaming(method.clientStreaming)
                .setServerStreaming(method.serverStreaming)
                .setExampleInput(method.comment.example);

        MessageDef input = index.messages.get(method.inputType);
        if (input != null) {
            for (FieldDef field : input.fields) {
                builder.addInputFields(toFieldDoc(field, index, new HashSet<>()));
            }
        }

        MessageDef output = index.messages.get(method.outputType);
        if (output != null) {
            for (FieldDef field : output.fields) {
                builder.addOutputFields(toFieldDoc(field, index, new HashSet<>()));
            }
        }

        return builder.build();
    }

    private static holons.v1.Describe.FieldDoc toFieldDoc(FieldDef field, ProtoIndex index, Set<String> seen) {
        holons.v1.Describe.FieldDoc.Builder builder = holons.v1.Describe.FieldDoc.newBuilder()
                .setName(field.name)
                .setType(field.typeName())
                .setNumber(field.number)
                .setDescription(field.comment.description)
                .setLabel(field.label())
                .setRequired(field.comment.required)
                .setExample(field.comment.example);

        if (field.mapKeyType != null) {
            builder.setMapKeyType(field.mapKeyType);
        }
        if (field.mapValueType != null) {
            builder.setMapValueType(field.mapValueType);
            String mapValueName = field.resolvedMapValueType(index);
            MessageDef nestedMessage = index.messages.get(mapValueName);
            if (nestedMessage != null && seen.add(nestedMessage.fullName)) {
                for (FieldDef nested : nestedMessage.fields) {
                    builder.addNestedFields(toFieldDoc(nested, index, new HashSet<>(seen)));
                }
            }
            EnumDef enumDef = index.enums.get(mapValueName);
            if (enumDef != null) {
                for (EnumValueDef value : enumDef.values) {
                    builder.addEnumValues(holons.v1.Describe.EnumValueDoc.newBuilder()
                            .setName(value.name)
                            .setNumber(value.number)
                            .setDescription(value.comment.description)
                            .build());
                }
            }
            return builder.build();
        }

        String resolvedType = field.resolvedType(index);
        MessageDef nestedMessage = index.messages.get(resolvedType);
        if (nestedMessage != null && seen.add(nestedMessage.fullName)) {
            for (FieldDef nested : nestedMessage.fields) {
                builder.addNestedFields(toFieldDoc(nested, index, new HashSet<>(seen)));
            }
        }

        EnumDef enumDef = index.enums.get(resolvedType);
        if (enumDef != null) {
            for (EnumValueDef value : enumDef.values) {
                builder.addEnumValues(holons.v1.Describe.EnumValueDoc.newBuilder()
                        .setName(value.name)
                        .setNumber(value.number)
                        .setDescription(value.comment.description)
                        .build());
            }
        }

        return builder.build();
    }

    private static ProtoIndex parseProtoDirectory(Path protoDir) throws IOException {
        if (!Files.isDirectory(protoDir)) {
            return new ProtoIndex();
        }

        List<Path> files;
        try (Stream<Path> walk = Files.walk(protoDir)) {
            files = walk
                    .filter(path -> Files.isRegularFile(path) && path.toString().endsWith(".proto"))
                    .sorted(Comparator.comparing(Path::toString))
                    .toList();
        }

        ProtoIndex index = new ProtoIndex();
        for (Path file : files) {
            parseProtoFile(file, index);
        }
        return index;
    }

    private static void parseProtoFile(Path file, ProtoIndex index) throws IOException {
        String pkg = "";
        Deque<Block> stack = new ArrayDeque<>();
        List<String> pendingComments = new ArrayList<>();

        for (String rawLine : Files.readAllLines(file)) {
            String line = rawLine.trim();
            if (line.startsWith("//")) {
                pendingComments.add(line.substring(2).trim());
                continue;
            }
            if (line.isEmpty()) {
                continue;
            }

            Matcher packageMatcher = PACKAGE_PATTERN.matcher(line);
            if (packageMatcher.find()) {
                pkg = packageMatcher.group(1);
                pendingComments.clear();
                continue;
            }

            Matcher serviceMatcher = SERVICE_PATTERN.matcher(line);
            if (serviceMatcher.find()) {
                String name = serviceMatcher.group(1);
                CommentMeta comment = CommentMeta.parse(pendingComments);
                pendingComments.clear();
                ServiceDef service = new ServiceDef(name, qualify(pkg, name), comment);
                index.services.add(service);
                stack.push(new Block(BlockKind.SERVICE, name, service, null, null));
                trimClosedBlocks(line, stack);
                continue;
            }

            Matcher messageMatcher = MESSAGE_PATTERN.matcher(line);
            if (messageMatcher.find()) {
                String name = messageMatcher.group(1);
                CommentMeta comment = CommentMeta.parse(pendingComments);
                pendingComments.clear();
                List<String> scope = messageScope(stack);
                MessageDef message = new MessageDef(name, qualify(pkg, qualifyScope(scope, name)), pkg, List.copyOf(scope), comment);
                index.messages.put(message.fullName, message);
                index.simpleTypes.putIfAbsent(message.simpleKey(), message.fullName);
                stack.push(new Block(BlockKind.MESSAGE, name, null, message, null));
                trimClosedBlocks(line, stack);
                continue;
            }

            Matcher enumMatcher = ENUM_PATTERN.matcher(line);
            if (enumMatcher.find()) {
                String name = enumMatcher.group(1);
                CommentMeta comment = CommentMeta.parse(pendingComments);
                pendingComments.clear();
                List<String> scope = messageScope(stack);
                EnumDef enumDef = new EnumDef(name, qualify(pkg, qualifyScope(scope, name)), pkg, List.copyOf(scope), comment);
                index.enums.put(enumDef.fullName, enumDef);
                index.simpleTypes.putIfAbsent(enumDef.simpleKey(), enumDef.fullName);
                stack.push(new Block(BlockKind.ENUM, name, null, null, enumDef));
                trimClosedBlocks(line, stack);
                continue;
            }

            Block current = stack.peek();
            if (current != null && current.kind == BlockKind.SERVICE) {
                Matcher rpcMatcher = RPC_PATTERN.matcher(line);
                if (rpcMatcher.find() && current.service != null) {
                    CommentMeta comment = CommentMeta.parse(pendingComments);
                    pendingComments.clear();
                    current.service.methods.add(new MethodDef(
                            rpcMatcher.group(1),
                            resolveTypeName(rpcMatcher.group(3), pkg, List.of(), index),
                            resolveTypeName(rpcMatcher.group(5), pkg, List.of(), index),
                            rpcMatcher.group(2) != null,
                            rpcMatcher.group(4) != null,
                            comment));
                    trimClosedBlocks(line, stack);
                    continue;
                }
            }

            if (current != null && current.kind == BlockKind.MESSAGE && current.message != null) {
                Matcher mapMatcher = MAP_FIELD_PATTERN.matcher(line);
                if (mapMatcher.find()) {
                    CommentMeta comment = CommentMeta.parse(pendingComments);
                    pendingComments.clear();
                    current.message.fields.add(new FieldDef(
                            mapMatcher.group(4),
                            Integer.parseInt(mapMatcher.group(5)),
                            comment,
                            FieldCardinality.MAP,
                            null,
                            resolveTypeName(mapMatcher.group(2), pkg, current.message.scope, index),
                            resolveTypeName(mapMatcher.group(3), pkg, current.message.scope, index),
                            pkg,
                            current.message.scope));
                    trimClosedBlocks(line, stack);
                    continue;
                }

                Matcher fieldMatcher = FIELD_PATTERN.matcher(line);
                if (fieldMatcher.find()) {
                    CommentMeta comment = CommentMeta.parse(pendingComments);
                    pendingComments.clear();
                    String qualifier = fieldMatcher.group(1) == null ? "" : fieldMatcher.group(1).trim();
                    FieldCardinality cardinality = "repeated".equals(qualifier)
                            ? FieldCardinality.REPEATED
                            : FieldCardinality.OPTIONAL;
                    current.message.fields.add(new FieldDef(
                            fieldMatcher.group(3),
                            Integer.parseInt(fieldMatcher.group(4)),
                            comment,
                            cardinality,
                            resolveTypeName(fieldMatcher.group(2), pkg, current.message.scope, index),
                            null,
                            null,
                            pkg,
                            current.message.scope));
                    trimClosedBlocks(line, stack);
                    continue;
                }
            }

            if (current != null && current.kind == BlockKind.ENUM && current.enumDef != null) {
                Matcher enumValueMatcher = ENUM_VALUE_PATTERN.matcher(line);
                if (enumValueMatcher.find()) {
                    CommentMeta comment = CommentMeta.parse(pendingComments);
                    pendingComments.clear();
                    current.enumDef.values.add(new EnumValueDef(
                            enumValueMatcher.group(1),
                            Integer.parseInt(enumValueMatcher.group(2)),
                            comment));
                    trimClosedBlocks(line, stack);
                    continue;
                }
            }

            pendingComments.clear();
            trimClosedBlocks(line, stack);
        }
    }

    private static void trimClosedBlocks(String line, Deque<Block> stack) {
        int closes = 0;
        for (int i = 0; i < line.length(); i++) {
            if (line.charAt(i) == '}') {
                closes++;
            }
        }
        while (closes-- > 0 && !stack.isEmpty()) {
            stack.pop();
        }
    }

    private static List<String> messageScope(Deque<Block> stack) {
        List<String> scope = new ArrayList<>();
        for (Block block : stack) {
            if (block.kind == BlockKind.MESSAGE) {
                scope.add(0, block.name);
            }
        }
        return scope;
    }

    private static String qualify(String pkg, String name) {
        if (name == null || name.isBlank()) {
            return "";
        }
        String cleaned = name.startsWith(".") ? name.substring(1) : name;
        if (cleaned.contains(".") || pkg == null || pkg.isBlank()) {
            return cleaned;
        }
        return pkg + "." + cleaned;
    }

    private static String qualifyScope(List<String> scope, String name) {
        if (scope.isEmpty()) {
            return name;
        }
        return String.join(".", scope) + "." + name;
    }

    private static String resolveTypeName(String typeName, String pkg, List<String> scope, ProtoIndex index) {
        if (typeName == null || typeName.isBlank()) {
            return "";
        }
        String cleaned = typeName.trim();
        if (cleaned.startsWith(".")) {
            return cleaned.substring(1);
        }
        if (SCALARS.contains(cleaned)) {
            return cleaned;
        }
        if (cleaned.contains(".")) {
            String qualified = qualify(pkg, cleaned);
            if (index.messages.containsKey(qualified) || index.enums.containsKey(qualified)) {
                return qualified;
            }
            return cleaned;
        }
        for (int i = scope.size(); i >= 0; i--) {
            List<String> prefix = scope.subList(0, i);
            String candidate = qualify(pkg, qualifyScope(prefix, cleaned));
            if (index.messages.containsKey(candidate) || index.enums.containsKey(candidate)) {
                return candidate;
            }
        }
        String simpleKey = qualifyScope(scope, cleaned);
        String matched = index.simpleTypes.get(simpleKey);
        if (matched != null) {
            return matched;
        }
        matched = index.simpleTypes.get(cleaned);
        if (matched != null) {
            return matched;
        }
        return qualify(pkg, cleaned);
    }

    private enum BlockKind {
        SERVICE,
        MESSAGE,
        ENUM
    }

    private enum FieldCardinality {
        OPTIONAL,
        REPEATED,
        MAP
    }

    private record Block(
            BlockKind kind,
            String name,
            ServiceDef service,
            MessageDef message,
            EnumDef enumDef) {
    }

    private static final class ProtoIndex {
        private final List<ServiceDef> services = new ArrayList<>();
        private final Map<String, MessageDef> messages = new LinkedHashMap<>();
        private final Map<String, EnumDef> enums = new LinkedHashMap<>();
        private final Map<String, String> simpleTypes = new HashMap<>();
    }

    private static final class ServiceDef {
        private final String fullName;
        private final CommentMeta comment;
        private final List<MethodDef> methods = new ArrayList<>();

        private ServiceDef(String name, String fullName, CommentMeta comment) {
            this.fullName = fullName;
            this.comment = comment;
        }
    }

    private record MethodDef(
            String name,
            String inputType,
            String outputType,
            boolean clientStreaming,
            boolean serverStreaming,
            CommentMeta comment) {
    }

    private static final class MessageDef {
        private final String fullName;
        private final String pkg;
        private final List<String> scope;
        private final CommentMeta comment;
        private final List<FieldDef> fields = new ArrayList<>();

        private MessageDef(String name, String fullName, String pkg, List<String> scope, CommentMeta comment) {
            this.fullName = fullName;
            this.pkg = pkg;
            this.scope = scope;
            this.comment = comment;
        }

        private String simpleKey() {
            return qualifyScope(scope, fullName.substring(fullName.lastIndexOf('.') + 1));
        }
    }

    private static final class EnumDef {
        private final String fullName;
        private final List<String> scope;
        private final List<EnumValueDef> values = new ArrayList<>();

        private EnumDef(String name, String fullName, String pkg, List<String> scope, CommentMeta comment) {
            this.fullName = fullName;
            this.scope = scope;
        }

        private String simpleKey() {
            return qualifyScope(scope, fullName.substring(fullName.lastIndexOf('.') + 1));
        }
    }

    private record EnumValueDef(String name, int number, CommentMeta comment) {
    }

    private static final class FieldDef {
        private final String name;
        private final int number;
        private final CommentMeta comment;
        private final FieldCardinality cardinality;
        private final String type;
        private final String mapKeyType;
        private final String mapValueType;
        private final String pkg;
        private final List<String> scope;

        private FieldDef(
                String name,
                int number,
                CommentMeta comment,
                FieldCardinality cardinality,
                String type,
                String mapKeyType,
                String mapValueType,
                String pkg,
                List<String> scope) {
            this.name = name;
            this.number = number;
            this.comment = comment;
            this.cardinality = cardinality;
            this.type = type;
            this.mapKeyType = mapKeyType;
            this.mapValueType = mapValueType;
            this.pkg = pkg;
            this.scope = scope;
        }

        private String typeName() {
            if (cardinality == FieldCardinality.MAP) {
                return "map<" + mapKeyType + ", " + mapValueType + ">";
            }
            return type;
        }

        private String resolvedType(ProtoIndex index) {
            return resolveTypeName(type, pkg, scope, index);
        }

        private String resolvedMapValueType(ProtoIndex index) {
            return resolveTypeName(mapValueType, pkg, scope, index);
        }

        private holons.v1.Describe.FieldLabel label() {
            return switch (cardinality) {
                case REPEATED -> holons.v1.Describe.FieldLabel.FIELD_LABEL_REPEATED;
                case MAP -> holons.v1.Describe.FieldLabel.FIELD_LABEL_MAP;
                case OPTIONAL -> holons.v1.Describe.FieldLabel.FIELD_LABEL_OPTIONAL;
            };
        }
    }

    private record CommentMeta(String description, boolean required, String example) {
        private static CommentMeta parse(List<String> lines) {
            List<String> description = new ArrayList<>();
            List<String> examples = new ArrayList<>();
            boolean required = false;

            for (String raw : lines) {
                String line = raw.trim();
                if (line.isEmpty()) {
                    continue;
                }
                if ("@required".equals(line)) {
                    required = true;
                    continue;
                }
                if (line.startsWith("@example")) {
                    String example = line.substring("@example".length()).trim();
                    if (!example.isEmpty()) {
                        examples.add(example);
                    }
                    continue;
                }
                description.add(line);
            }

            return new CommentMeta(
                    String.join(" ", description),
                    required,
                    String.join("\n", examples));
        }
    }
}
