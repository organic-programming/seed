using System.Text.RegularExpressions;
using Google.Protobuf;
using Grpc.Core;
using Holons.V1;

namespace Holons;

/// <summary>Build-time describe generation helpers and runtime static HolonMeta registration.</summary>
public static class Describe
{
    private const string HolonMetaService = "holons.v1.HolonMeta";
    public const string NoIncodeDescriptionMessage = "no Incode Description registered — run op build";

    private static readonly Regex PackagePattern = new(@"^package\s+([A-Za-z0-9_.]+)\s*;", RegexOptions.Compiled);
    private static readonly Regex ServicePattern = new(@"^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?", RegexOptions.Compiled);
    private static readonly Regex MessagePattern = new(@"^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?", RegexOptions.Compiled);
    private static readonly Regex EnumPattern = new(@"^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?", RegexOptions.Compiled);
    private static readonly Regex RpcPattern = new(
        @"^rpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)",
        RegexOptions.Compiled);
    private static readonly Regex MapFieldPattern = new(
        @"^(repeated\s+)?map\s*<\s*([.A-Za-z0-9_]+)\s*,\s*([.A-Za-z0-9_]+)\s*>\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;",
        RegexOptions.Compiled);
    private static readonly Regex FieldPattern = new(
        @"^(optional\s+|repeated\s+)?([.A-Za-z0-9_]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;",
        RegexOptions.Compiled);
    private static readonly Regex EnumValuePattern = new(
        @"^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(-?\d+)\s*;",
        RegexOptions.Compiled);
    private static readonly HashSet<string> Scalars = new(StringComparer.Ordinal)
    {
        "double", "float", "int64", "uint64", "int32", "fixed64", "fixed32",
        "bool", "string", "bytes", "uint32", "sfixed32", "sfixed64",
        "sint32", "sint64"
    };
    private static readonly object StaticResponseGate = new();
    private static DescribeResponse? StaticResponse;

    public static readonly Method<DescribeRequest, DescribeResponse> DescribeMethod =
        new(
            MethodType.Unary,
            HolonMetaService,
            "Describe",
            Marshallers.Create(
                message => message.ToByteArray(),
                DescribeRequest.Parser.ParseFrom),
            Marshallers.Create(
                message => message.ToByteArray(),
                DescribeResponse.Parser.ParseFrom));

    public static void UseStaticResponse(DescribeResponse? response)
    {
        lock (StaticResponseGate)
        {
            StaticResponse = CloneDescribeResponse(response);
        }
    }

    public static ServerServiceDefinition BindService()
    {
        var response = RequireStaticResponse();
        return ServerServiceDefinition.CreateBuilder()
            .AddMethod(
                DescribeMethod,
                new UnaryServerMethod<DescribeRequest, DescribeResponse>((_, _) => Task.FromResult(response)))
            .Build();
    }

    internal static Serve.GrpcServiceRegistration Registration()
    {
        return Serve.Service(new DescribeGrpcService(RequireStaticResponse()));
    }

    public static DescribeResponse BuildResponse(string protoDir)
    {
        var resolved = Identity.Resolve(protoDir);
        var index = ParseProtoDirectory(protoDir);

        var response = new DescribeResponse
        {
            Manifest = ProtoManifest(resolved),
        };

        foreach (var service in index.Services)
        {
            if (string.Equals(service.FullName, HolonMetaService, StringComparison.Ordinal))
                continue;
            response.Services.Add(ServiceDoc(service, index));
        }

        return response;
    }

    private static DescribeResponse RequireStaticResponse()
    {
        lock (StaticResponseGate)
        {
            return CloneDescribeResponse(StaticResponse)
                ?? throw new InvalidOperationException(NoIncodeDescriptionMessage);
        }
    }

    private static DescribeResponse? CloneDescribeResponse(DescribeResponse? response)
    {
        return response?.Clone();
    }

    private static HolonManifest ProtoManifest(IdentityParser.ResolvedManifest resolved)
    {
        return new HolonManifest
        {
            Identity = new HolonManifest.Types.Identity
            {
                Uuid = resolved.Identity.Uuid,
                GivenName = resolved.Identity.GivenName,
                FamilyName = resolved.Identity.FamilyName,
                Motto = resolved.Identity.Motto,
                Composer = resolved.Identity.Composer,
                Status = resolved.Identity.Status,
                Born = resolved.Identity.Born,
            },
            Lang = resolved.Identity.Lang,
            Kind = resolved.Kind,
            Build = new HolonManifest.Types.Build
            {
                Runner = resolved.BuildRunner,
                Main = resolved.BuildMain,
            },
            Artifacts = new HolonManifest.Types.Artifacts
            {
                Binary = resolved.ArtifactBinary,
                Primary = resolved.ArtifactPrimary,
            },
        };
    }

    private static ServiceDoc ServiceDoc(ServiceDef service, ProtoIndex index)
    {
        var doc = new ServiceDoc
        {
            Name = service.FullName,
            Description = service.Comment.Description,
        };

        foreach (var method in service.Methods)
            doc.Methods.Add(MethodDoc(method, index));

        return doc;
    }

    private static MethodDoc MethodDoc(MethodDef method, ProtoIndex index)
    {
        var doc = new MethodDoc
        {
            Name = method.Name,
            Description = method.Comment.Description,
            InputType = method.InputType,
            OutputType = method.OutputType,
            ClientStreaming = method.ClientStreaming,
            ServerStreaming = method.ServerStreaming,
            ExampleInput = method.Comment.Example,
        };

        if (index.Messages.TryGetValue(method.InputType, out var input))
        {
            foreach (var field in input.Fields)
                doc.InputFields.Add(FieldDoc(field, index, new HashSet<string>(StringComparer.Ordinal)));
        }

        if (index.Messages.TryGetValue(method.OutputType, out var output))
        {
            foreach (var field in output.Fields)
                doc.OutputFields.Add(FieldDoc(field, index, new HashSet<string>(StringComparer.Ordinal)));
        }

        return doc;
    }

    private static FieldDoc FieldDoc(FieldDef field, ProtoIndex index, HashSet<string> seen)
    {
        var doc = new FieldDoc
        {
            Name = field.Name,
            Type = field.TypeName(),
            Number = field.Number,
            Description = field.Comment.Description,
            Label = field.Label(),
            Required = field.Comment.Required,
            Example = field.Comment.Example,
        };

        if (!string.IsNullOrEmpty(field.MapKeyType))
            doc.MapKeyType = field.MapKeyType;
        if (!string.IsNullOrEmpty(field.MapValueType))
            doc.MapValueType = field.MapValueType;

        if (field.Cardinality == FieldCardinality.Map)
        {
            var mapValueType = field.ResolvedMapValueType(index);
            if (index.Messages.TryGetValue(mapValueType, out var nested) && seen.Add(nested.FullName))
            {
                foreach (var nestedField in nested.Fields)
                    doc.NestedFields.Add(FieldDoc(nestedField, index, new HashSet<string>(seen, StringComparer.Ordinal)));
            }

            if (index.Enums.TryGetValue(mapValueType, out var enumDef))
            {
                foreach (var value in enumDef.Values)
                    doc.EnumValues.Add(EnumValueDoc(value));
            }

            return doc;
        }

        var resolvedType = field.ResolvedType(index);
        if (index.Messages.TryGetValue(resolvedType, out var nestedMessage) && seen.Add(nestedMessage.FullName))
        {
            foreach (var nestedField in nestedMessage.Fields)
                doc.NestedFields.Add(FieldDoc(nestedField, index, new HashSet<string>(seen, StringComparer.Ordinal)));
        }

        if (index.Enums.TryGetValue(resolvedType, out var enumType))
        {
            foreach (var value in enumType.Values)
                doc.EnumValues.Add(EnumValueDoc(value));
        }

        return doc;
    }

    private static EnumValueDoc EnumValueDoc(EnumValueDef value) =>
        new()
        {
            Name = value.Name,
            Number = value.Number,
            Description = value.Comment.Description,
        };

    private static ProtoIndex ParseProtoDirectory(string protoDir)
    {
        var index = new ProtoIndex();
        if (string.IsNullOrWhiteSpace(protoDir) || !Directory.Exists(protoDir))
            return index;

        foreach (var file in Directory.EnumerateFiles(protoDir, "*.proto", SearchOption.AllDirectories).OrderBy(path => path, StringComparer.Ordinal))
            ParseProtoFile(file, index);

        return index;
    }

    private static void ParseProtoFile(string path, ProtoIndex index)
    {
        var pkg = "";
        var stack = new Stack<Block>();
        var pendingComments = new List<string>();

        foreach (var rawLine in File.ReadAllLines(path))
        {
            var line = rawLine.Trim();
            if (line.StartsWith("//", StringComparison.Ordinal))
            {
                pendingComments.Add(line[2..].Trim());
                continue;
            }

            if (string.IsNullOrWhiteSpace(line))
                continue;

            var packageMatch = PackagePattern.Match(line);
            if (packageMatch.Success)
            {
                pkg = packageMatch.Groups[1].Value;
                pendingComments.Clear();
                continue;
            }

            var serviceMatch = ServicePattern.Match(line);
            if (serviceMatch.Success)
            {
                var service = new ServiceDef(Qualify(pkg, serviceMatch.Groups[1].Value), CommentMeta.Parse(pendingComments));
                index.Services.Add(service);
                pendingComments.Clear();
                stack.Push(new Block(BlockKind.Service, serviceMatch.Groups[1].Value, Service: service));
                TrimClosedBlocks(line, stack);
                continue;
            }

            var messageMatch = MessagePattern.Match(line);
            if (messageMatch.Success)
            {
                var scope = MessageScope(stack);
                var message = new MessageDef(
                    Qualify(pkg, QualifyScope(scope, messageMatch.Groups[1].Value)),
                    pkg,
                    scope,
                    CommentMeta.Parse(pendingComments));
                index.Messages[message.FullName] = message;
                index.SimpleTypes.TryAdd(message.SimpleKey(), message.FullName);
                pendingComments.Clear();
                stack.Push(new Block(BlockKind.Message, messageMatch.Groups[1].Value, Message: message));
                TrimClosedBlocks(line, stack);
                continue;
            }

            var enumMatch = EnumPattern.Match(line);
            if (enumMatch.Success)
            {
                var scope = MessageScope(stack);
                var enumDef = new EnumDef(Qualify(pkg, QualifyScope(scope, enumMatch.Groups[1].Value)), scope);
                index.Enums[enumDef.FullName] = enumDef;
                index.SimpleTypes.TryAdd(enumDef.SimpleKey(), enumDef.FullName);
                pendingComments.Clear();
                stack.Push(new Block(BlockKind.Enum, enumMatch.Groups[1].Value, EnumDef: enumDef));
                TrimClosedBlocks(line, stack);
                continue;
            }

            if (stack.TryPeek(out var current))
            {
                if (current.Kind == BlockKind.Service && current.Service is not null)
                {
                    var rpcMatch = RpcPattern.Match(line);
                    if (rpcMatch.Success)
                    {
                        current.Service.Methods.Add(new MethodDef(
                            rpcMatch.Groups[1].Value,
                            ResolveTypeName(rpcMatch.Groups[3].Value, pkg, [], index),
                            ResolveTypeName(rpcMatch.Groups[5].Value, pkg, [], index),
                            rpcMatch.Groups[2].Success,
                            rpcMatch.Groups[4].Success,
                            CommentMeta.Parse(pendingComments)));
                        pendingComments.Clear();
                        TrimClosedBlocks(line, stack);
                        continue;
                    }
                }

                if (current.Kind == BlockKind.Message && current.Message is not null)
                {
                    var mapFieldMatch = MapFieldPattern.Match(line);
                    if (mapFieldMatch.Success)
                    {
                        current.Message.Fields.Add(new FieldDef(
                            mapFieldMatch.Groups[4].Value,
                            int.Parse(mapFieldMatch.Groups[5].Value),
                            CommentMeta.Parse(pendingComments),
                            FieldCardinality.Map,
                            null,
                            ResolveTypeName(mapFieldMatch.Groups[2].Value, pkg, current.Message.Scope, index),
                            ResolveTypeName(mapFieldMatch.Groups[3].Value, pkg, current.Message.Scope, index),
                            pkg,
                            current.Message.Scope));
                        pendingComments.Clear();
                        TrimClosedBlocks(line, stack);
                        continue;
                    }

                    var fieldMatch = FieldPattern.Match(line);
                    if (fieldMatch.Success)
                    {
                        var qualifier = fieldMatch.Groups[1].Value.Trim();
                        current.Message.Fields.Add(new FieldDef(
                            fieldMatch.Groups[3].Value,
                            int.Parse(fieldMatch.Groups[4].Value),
                            CommentMeta.Parse(pendingComments),
                            qualifier == "repeated" ? FieldCardinality.Repeated : FieldCardinality.Optional,
                            ResolveTypeName(fieldMatch.Groups[2].Value, pkg, current.Message.Scope, index),
                            null,
                            null,
                            pkg,
                            current.Message.Scope));
                        pendingComments.Clear();
                        TrimClosedBlocks(line, stack);
                        continue;
                    }
                }

                if (current.Kind == BlockKind.Enum && current.EnumDef is not null)
                {
                    var enumValueMatch = EnumValuePattern.Match(line);
                    if (enumValueMatch.Success)
                    {
                        current.EnumDef.Values.Add(new EnumValueDef(
                            enumValueMatch.Groups[1].Value,
                            int.Parse(enumValueMatch.Groups[2].Value),
                            CommentMeta.Parse(pendingComments)));
                        pendingComments.Clear();
                        TrimClosedBlocks(line, stack);
                        continue;
                    }
                }
            }

            pendingComments.Clear();
            TrimClosedBlocks(line, stack);
        }
    }

    private static void TrimClosedBlocks(string line, Stack<Block> stack)
    {
        foreach (var _ in line.Where(ch => ch == '}'))
        {
            if (stack.Count > 0)
                stack.Pop();
        }
    }

    private static List<string> MessageScope(IEnumerable<Block> stack) =>
        stack
            .Where(block => block.Kind == BlockKind.Message)
            .Select(block => block.Name)
            .Reverse()
            .ToList();

    private static string Qualify(string pkg, string name)
    {
        if (string.IsNullOrWhiteSpace(name))
            return "";

        var cleaned = name.StartsWith(".", StringComparison.Ordinal) ? name[1..] : name;
        if (cleaned.Contains(".", StringComparison.Ordinal) || string.IsNullOrWhiteSpace(pkg))
            return cleaned;
        return $"{pkg}.{cleaned}";
    }

    private static string QualifyScope(IReadOnlyList<string> scope, string name) =>
        scope.Count == 0 ? name : $"{string.Join(".", scope)}.{name}";

    private static string ResolveTypeName(string typeName, string pkg, IReadOnlyList<string> scope, ProtoIndex index)
    {
        if (string.IsNullOrWhiteSpace(typeName))
            return "";

        var cleaned = typeName.Trim();
        if (cleaned.StartsWith(".", StringComparison.Ordinal))
            return cleaned[1..];
        if (Scalars.Contains(cleaned))
            return cleaned;

        if (cleaned.Contains(".", StringComparison.Ordinal))
        {
            var qualified = Qualify(pkg, cleaned);
            return index.Messages.ContainsKey(qualified) || index.Enums.ContainsKey(qualified)
                ? qualified
                : cleaned;
        }

        for (var i = scope.Count; i >= 0; i--)
        {
            var candidate = Qualify(pkg, QualifyScope(scope.Take(i).ToList(), cleaned));
            if (index.Messages.ContainsKey(candidate) || index.Enums.ContainsKey(candidate))
                return candidate;
        }

        if (index.SimpleTypes.TryGetValue(QualifyScope(scope, cleaned), out var nestedMatch))
            return nestedMatch;
        if (index.SimpleTypes.TryGetValue(cleaned, out var directMatch))
            return directMatch;

        return Qualify(pkg, cleaned);
    }

    private enum BlockKind
    {
        Service,
        Message,
        Enum,
    }

    private enum FieldCardinality
    {
        Optional,
        Repeated,
        Map,
    }

    private sealed record Block(
        BlockKind Kind,
        string Name,
        ServiceDef? Service = null,
        MessageDef? Message = null,
        EnumDef? EnumDef = null);

    private sealed class ProtoIndex
    {
        public List<ServiceDef> Services { get; } = [];
        public Dictionary<string, MessageDef> Messages { get; } = new(StringComparer.Ordinal);
        public Dictionary<string, EnumDef> Enums { get; } = new(StringComparer.Ordinal);
        public Dictionary<string, string> SimpleTypes { get; } = new(StringComparer.Ordinal);
    }

    private sealed class ServiceDef
    {
        public ServiceDef(string fullName, CommentMeta comment)
        {
            FullName = fullName;
            Comment = comment;
        }

        public string FullName { get; }
        public CommentMeta Comment { get; }
        public List<MethodDef> Methods { get; } = [];
    }

    private sealed record MethodDef(
        string Name,
        string InputType,
        string OutputType,
        bool ClientStreaming,
        bool ServerStreaming,
        CommentMeta Comment);

    private sealed class MessageDef
    {
        public MessageDef(string fullName, string pkg, IReadOnlyList<string> scope, CommentMeta comment)
        {
            FullName = fullName;
            Package = pkg;
            Scope = scope;
            Comment = comment;
        }

        public string FullName { get; }
        public string Package { get; }
        public IReadOnlyList<string> Scope { get; }
        public CommentMeta Comment { get; }
        public List<FieldDef> Fields { get; } = [];

        public string SimpleKey() => QualifyScope(Scope, FullName[(FullName.LastIndexOf('.') + 1)..]);
    }

    private sealed class EnumDef
    {
        public EnumDef(string fullName, IReadOnlyList<string> scope)
        {
            FullName = fullName;
            Scope = scope;
        }

        public string FullName { get; }
        public IReadOnlyList<string> Scope { get; }
        public List<EnumValueDef> Values { get; } = [];

        public string SimpleKey() => QualifyScope(Scope, FullName[(FullName.LastIndexOf('.') + 1)..]);
    }

    private sealed record EnumValueDef(string Name, int Number, CommentMeta Comment);

    private sealed class FieldDef
    {
        public FieldDef(
            string name,
            int number,
            CommentMeta comment,
            FieldCardinality cardinality,
            string? type,
            string? mapKeyType,
            string? mapValueType,
            string packageName,
            IReadOnlyList<string> scope)
        {
            Name = name;
            Number = number;
            Comment = comment;
            Cardinality = cardinality;
            Type = type;
            MapKeyType = mapKeyType;
            MapValueType = mapValueType;
            PackageName = packageName;
            Scope = scope;
        }

        public string Name { get; }
        public int Number { get; }
        public CommentMeta Comment { get; }
        public FieldCardinality Cardinality { get; }
        public string? Type { get; }
        public string? MapKeyType { get; }
        public string? MapValueType { get; }
        public string PackageName { get; }
        public IReadOnlyList<string> Scope { get; }

        public string TypeName() =>
            Cardinality == FieldCardinality.Map
                ? $"map<{MapKeyType}, {MapValueType}>"
                : Type ?? "";

        public string ResolvedType(ProtoIndex index) =>
            ResolveTypeName(Type ?? "", PackageName, Scope, index);

        public string ResolvedMapValueType(ProtoIndex index) =>
            ResolveTypeName(MapValueType ?? "", PackageName, Scope, index);

        public FieldLabel Label() =>
            Cardinality switch
            {
                FieldCardinality.Repeated => FieldLabel.Repeated,
                FieldCardinality.Map => FieldLabel.Map,
                _ => FieldLabel.Optional,
            };
    }

    private sealed record CommentMeta(string Description, bool Required, string Example)
    {
        public static CommentMeta Parse(IEnumerable<string> lines)
        {
            var description = new List<string>();
            var examples = new List<string>();
            var required = false;

            foreach (var raw in lines)
            {
                var line = raw.Trim();
                if (line.Length == 0)
                    continue;
                if (string.Equals(line, "@required", StringComparison.Ordinal))
                {
                    required = true;
                    continue;
                }
                if (line.StartsWith("@example", StringComparison.Ordinal))
                {
                    var example = line["@example".Length..].Trim();
                    if (example.Length > 0)
                        examples.Add(example);
                    continue;
                }
                description.Add(line);
            }

            return new CommentMeta(
                string.Join(" ", description),
                required,
                string.Join("\n", examples));
        }
    }

    private sealed class DescribeGrpcService : HolonMeta.HolonMetaBase
    {
        private readonly DescribeResponse _response;

        public DescribeGrpcService(DescribeResponse response)
        {
            _response = response;
        }

        public override Task<DescribeResponse> Describe(DescribeRequest request, ServerCallContext context)
        {
            _ = request;
            _ = context;
            return Task.FromResult(_response);
        }
    }
}
