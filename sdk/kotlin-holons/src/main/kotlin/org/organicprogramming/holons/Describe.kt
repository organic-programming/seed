package org.organicprogramming.holons

import holons.v1.Describe as HolonsDescribe
import holons.v1.Manifest as HolonsManifest
import io.grpc.MethodDescriptor
import io.grpc.ServerServiceDefinition
import io.grpc.protobuf.ProtoUtils
import io.grpc.stub.ServerCalls
import java.nio.file.Files
import java.nio.file.Path
import java.util.ArrayDeque
import kotlin.io.path.exists
import kotlin.io.path.isDirectory

/** Build-time describe helpers plus runtime registration for a static HolonMeta response. */
object Describe {
    private const val HOLON_META_SERVICE = "holons.v1.HolonMeta"
    const val NO_INCODE_DESCRIPTION_MESSAGE = "no Incode Description registered — run op build"
    private val packagePattern = Regex("""^package\s+([A-Za-z0-9_.]+)\s*;""")
    private val servicePattern = Regex("""^service\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?""")
    private val messagePattern = Regex("""^message\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?""")
    private val enumPattern = Regex("""^enum\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{?""")
    private val rpcPattern =
        Regex("""^rpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([.A-Za-z0-9_]+)\s*\)\s*;?""")
    private val mapFieldPattern =
        Regex("""^(repeated\s+)?map\s*<\s*([.A-Za-z0-9_]+)\s*,\s*([.A-Za-z0-9_]+)\s*>\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;""")
    private val fieldPattern =
        Regex("""^(optional\s+|repeated\s+)?([.A-Za-z0-9_]+)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(\d+)\s*;""")
    private val enumValuePattern = Regex("""^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(-?\d+)\s*;""")
    private val scalars = setOf(
        "double", "float", "int64", "uint64", "int32", "fixed64", "fixed32",
        "bool", "string", "bytes", "uint32", "sfixed32", "sfixed64",
        "sint32", "sint64",
    )

    private val describeMethodDescriptor: MethodDescriptor<HolonsDescribe.DescribeRequest, HolonsDescribe.DescribeResponse> =
        MethodDescriptor.newBuilder<HolonsDescribe.DescribeRequest, HolonsDescribe.DescribeResponse>()
            .setType(MethodDescriptor.MethodType.UNARY)
            .setFullMethodName(MethodDescriptor.generateFullMethodName(HOLON_META_SERVICE, "Describe"))
            .setRequestMarshaller(ProtoUtils.marshaller(HolonsDescribe.DescribeRequest.getDefaultInstance()))
            .setResponseMarshaller(ProtoUtils.marshaller(HolonsDescribe.DescribeResponse.getDefaultInstance()))
            .build()

    @Volatile
    private var staticResponse: HolonsDescribe.DescribeResponse? = null

    fun describeMethod(): MethodDescriptor<HolonsDescribe.DescribeRequest, HolonsDescribe.DescribeResponse> = describeMethodDescriptor

    fun useStaticResponse(response: HolonsDescribe.DescribeResponse?) {
        staticResponse = cloneResponse(response)
    }

    fun staticResponse(): HolonsDescribe.DescribeResponse? = cloneResponse(staticResponse)

    fun serviceDefinition(): ServerServiceDefinition {
        val response = registeredStaticResponse()
        return ServerServiceDefinition.builder(HOLON_META_SERVICE)
            .addMethod(
                describeMethodDescriptor,
                ServerCalls.asyncUnaryCall<HolonsDescribe.DescribeRequest, HolonsDescribe.DescribeResponse> { _, observer ->
                    observer.onNext(response)
                    observer.onCompleted()
                },
            )
            .build()
    }

    private fun registeredStaticResponse(): HolonsDescribe.DescribeResponse =
        cloneResponse(staticResponse) ?: throw IllegalStateException(NO_INCODE_DESCRIPTION_MESSAGE)

    private fun cloneResponse(response: HolonsDescribe.DescribeResponse?): HolonsDescribe.DescribeResponse? =
        response?.toBuilder()?.build()

    fun buildResponse(protoDir: Path): HolonsDescribe.DescribeResponse {
        val resolved = Identity.parseManifest(Identity.resolveManifestPath(protoDir))
        val index = parseProtoDirectory(protoDir)

        val builder = HolonsDescribe.DescribeResponse.newBuilder()
            .setManifest(protoManifest(resolved))

        index.services
            .filterNot { it.fullName == HOLON_META_SERVICE }
            .forEach { builder.addServices(serviceDoc(it, index)) }

        return builder.build()
    }

    private fun protoManifest(resolved: ResolvedManifest): HolonsManifest.HolonManifest =
        HolonsManifest.HolonManifest.newBuilder()
            .setIdentity(
                HolonsManifest.HolonManifest.Identity.newBuilder()
                    .setUuid(resolved.identity.uuid)
                    .setGivenName(resolved.identity.givenName)
                    .setFamilyName(resolved.identity.familyName)
                    .setMotto(resolved.identity.motto)
                    .setComposer(resolved.identity.composer)
                    .setStatus(resolved.identity.status)
                    .setBorn(resolved.identity.born)
                    .addAllAliases(resolved.identity.aliases),
            )
            .setLang(resolved.identity.lang)
            .setKind(resolved.kind)
            .setBuild(
                HolonsManifest.HolonManifest.Build.newBuilder()
                    .setRunner(resolved.buildRunner)
                    .setMain(resolved.buildMain),
            )
            .setArtifacts(
                HolonsManifest.HolonManifest.Artifacts.newBuilder()
                    .setBinary(resolved.artifactBinary)
                    .setPrimary(resolved.artifactPrimary),
            )
            .build()

    private fun serviceDoc(service: ServiceDef, index: ProtoIndex): HolonsDescribe.ServiceDoc =
        HolonsDescribe.ServiceDoc.newBuilder()
            .setName(service.fullName)
            .setDescription(service.comment.description)
            .apply {
                service.methods.forEach { addMethods(methodDoc(it, index)) }
            }
            .build()

    private fun methodDoc(method: MethodDef, index: ProtoIndex): HolonsDescribe.MethodDoc {
        val builder = HolonsDescribe.MethodDoc.newBuilder()
            .setName(method.name)
            .setDescription(method.comment.description)
            .setInputType(method.inputType)
            .setOutputType(method.outputType)
            .setClientStreaming(method.clientStreaming)
            .setServerStreaming(method.serverStreaming)
            .setExampleInput(method.comment.example)

        index.messages[method.inputType]?.fields?.forEach { builder.addInputFields(fieldDoc(it, index, mutableSetOf())) }
        index.messages[method.outputType]?.fields?.forEach { builder.addOutputFields(fieldDoc(it, index, mutableSetOf())) }

        return builder.build()
    }

    private fun fieldDoc(
        field: FieldDef,
        index: ProtoIndex,
        seen: MutableSet<String>,
    ): HolonsDescribe.FieldDoc {
        val builder = HolonsDescribe.FieldDoc.newBuilder()
            .setName(field.name)
            .setType(field.typeName())
            .setNumber(field.number)
            .setDescription(field.comment.description)
            .setLabel(field.label())
            .setRequired(field.comment.required)
            .setExample(field.comment.example)

        field.mapKeyType?.let(builder::setMapKeyType)
        field.mapValueType?.let(builder::setMapValueType)

        if (field.cardinality == FieldCardinality.MAP) {
            val mapValueType = field.resolvedMapValueType(index)
            index.messages[mapValueType]?.let { nested ->
                if (seen.add(nested.fullName)) {
                    nested.fields.forEach { builder.addNestedFields(fieldDoc(it, index, seen.toMutableSet())) }
                }
            }
            index.enums[mapValueType]?.values?.forEach { builder.addEnumValues(enumValueDoc(it)) }
            return builder.build()
        }

        val resolvedType = field.resolvedType(index)
        index.messages[resolvedType]?.let { nested ->
            if (seen.add(nested.fullName)) {
                nested.fields.forEach { builder.addNestedFields(fieldDoc(it, index, seen.toMutableSet())) }
            }
        }
        index.enums[resolvedType]?.values?.forEach { builder.addEnumValues(enumValueDoc(it)) }

        return builder.build()
    }

    private fun enumValueDoc(value: EnumValueDef): HolonsDescribe.EnumValueDoc =
        HolonsDescribe.EnumValueDoc.newBuilder()
            .setName(value.name)
            .setNumber(value.number)
            .setDescription(value.comment.description)
            .build()

    private fun parseProtoDirectory(protoDir: Path): ProtoIndex {
        if (!protoDir.exists() || !protoDir.isDirectory()) {
            return ProtoIndex()
        }

        val files = Files.walk(protoDir).use { walk ->
            walk
                .filter { Files.isRegularFile(it) && it.toString().endsWith(".proto") }
                .sorted(compareBy(Path::toString))
                .toList()
        }

        val index = ProtoIndex()
        files.forEach { parseProtoFile(it, index) }
        return index
    }

    private fun parseProtoFile(file: Path, index: ProtoIndex) {
        var pkg = ""
        val stack = ArrayDeque<Block>()
        val pendingComments = mutableListOf<String>()

        Files.readAllLines(file).forEach { raw ->
            val line = raw.trim()
            if (line.startsWith("//")) {
                pendingComments += line.removePrefix("//").trim()
                return@forEach
            }
            if (line.isEmpty()) {
                return@forEach
            }

            packagePattern.matchEntire(line)?.let { match ->
                pkg = match.groupValues[1]
                pendingComments.clear()
                return@forEach
            }

            servicePattern.matchEntire(line)?.let { match ->
                val name = match.groupValues[1]
                val service = ServiceDef(qualify(pkg, name), CommentMeta.parse(pendingComments))
                index.services += service
                pendingComments.clear()
                stack.push(Block(BlockKind.SERVICE, name, service = service))
                trimClosedBlocks(line, stack)
                return@forEach
            }

            messagePattern.matchEntire(line)?.let { match ->
                val name = match.groupValues[1]
                val scope = messageScope(stack)
                val message = MessageDef(
                    fullName = qualify(pkg, qualifyScope(scope, name)),
                    pkg = pkg,
                    scope = scope,
                    comment = CommentMeta.parse(pendingComments),
                )
                index.messages[message.fullName] = message
                index.simpleTypes.putIfAbsent(message.simpleKey(), message.fullName)
                pendingComments.clear()
                stack.push(Block(BlockKind.MESSAGE, name, message = message))
                trimClosedBlocks(line, stack)
                return@forEach
            }

            enumPattern.matchEntire(line)?.let { match ->
                val name = match.groupValues[1]
                val scope = messageScope(stack)
                val enumDef = EnumDef(
                    fullName = qualify(pkg, qualifyScope(scope, name)),
                    scope = scope,
                )
                index.enums[enumDef.fullName] = enumDef
                index.simpleTypes.putIfAbsent(enumDef.simpleKey(), enumDef.fullName)
                pendingComments.clear()
                stack.push(Block(BlockKind.ENUM, name, enumDef = enumDef))
                trimClosedBlocks(line, stack)
                return@forEach
            }

            when (val current = stack.peek()) {
                null -> {
                    pendingComments.clear()
                    trimClosedBlocks(line, stack)
                }

                else -> when (current.kind) {
                    BlockKind.SERVICE -> {
                        rpcPattern.matchEntire(line)?.let { match ->
                            current.service?.methods?.add(
                                MethodDef(
                                    name = match.groupValues[1],
                                    inputType = resolveTypeName(match.groupValues[3], pkg, emptyList(), index),
                                    outputType = resolveTypeName(match.groupValues[5], pkg, emptyList(), index),
                                    clientStreaming = match.groupValues[2].isNotEmpty(),
                                    serverStreaming = match.groupValues[4].isNotEmpty(),
                                    comment = CommentMeta.parse(pendingComments),
                                ),
                            )
                            pendingComments.clear()
                            trimClosedBlocks(line, stack)
                            return@forEach
                        }
                        pendingComments.clear()
                        trimClosedBlocks(line, stack)
                    }

                    BlockKind.MESSAGE -> {
                        mapFieldPattern.matchEntire(line)?.let { match ->
                            current.message?.fields?.add(
                                FieldDef(
                                    name = match.groupValues[4],
                                    number = match.groupValues[5].toInt(),
                                    comment = CommentMeta.parse(pendingComments),
                                    cardinality = FieldCardinality.MAP,
                                    type = null,
                                    mapKeyType = resolveTypeName(match.groupValues[2], pkg, current.message.scope, index),
                                    mapValueType = resolveTypeName(match.groupValues[3], pkg, current.message.scope, index),
                                    pkg = pkg,
                                    scope = current.message.scope,
                                ),
                            )
                            pendingComments.clear()
                            trimClosedBlocks(line, stack)
                            return@forEach
                        }

                        fieldPattern.matchEntire(line)?.let { match ->
                            val qualifier = match.groupValues[1].trim()
                            val cardinality = if (qualifier == "repeated") {
                                FieldCardinality.REPEATED
                            } else {
                                FieldCardinality.OPTIONAL
                            }
                            current.message?.fields?.add(
                                FieldDef(
                                    name = match.groupValues[3],
                                    number = match.groupValues[4].toInt(),
                                    comment = CommentMeta.parse(pendingComments),
                                    cardinality = cardinality,
                                    type = resolveTypeName(match.groupValues[2], pkg, current.message.scope, index),
                                    mapKeyType = null,
                                    mapValueType = null,
                                    pkg = pkg,
                                    scope = current.message.scope,
                                ),
                            )
                            pendingComments.clear()
                            trimClosedBlocks(line, stack)
                            return@forEach
                        }
                        pendingComments.clear()
                        trimClosedBlocks(line, stack)
                    }

                    BlockKind.ENUM -> {
                        enumValuePattern.matchEntire(line)?.let { match ->
                            current.enumDef?.values?.add(
                                EnumValueDef(
                                    name = match.groupValues[1],
                                    number = match.groupValues[2].toInt(),
                                    comment = CommentMeta.parse(pendingComments),
                                ),
                            )
                            pendingComments.clear()
                            trimClosedBlocks(line, stack)
                            return@forEach
                        }
                        pendingComments.clear()
                        trimClosedBlocks(line, stack)
                    }
                }
            }
        }
    }

    private fun trimClosedBlocks(line: String, stack: ArrayDeque<Block>) {
        repeat(line.count { it == '}' }) {
            if (stack.isNotEmpty()) {
                stack.pop()
            }
        }
    }

    private fun messageScope(stack: ArrayDeque<Block>): List<String> =
        stack
            .filter { it.kind == BlockKind.MESSAGE }
            .map { it.name }
            .reversed()

    private fun qualify(pkg: String, name: String): String {
        if (name.isBlank()) {
            return ""
        }
        val cleaned = name.removePrefix(".")
        return if (cleaned.contains(".") || pkg.isBlank()) cleaned else "$pkg.$cleaned"
    }

    private fun qualifyScope(scope: List<String>, name: String): String =
        if (scope.isEmpty()) name else scope.joinToString(".") + "." + name

    private fun resolveTypeName(
        typeName: String,
        pkg: String,
        scope: List<String>,
        index: ProtoIndex,
    ): String {
        if (typeName.isBlank()) {
            return ""
        }
        val cleaned = typeName.trim()
        if (cleaned.startsWith(".")) {
            return cleaned.removePrefix(".")
        }
        if (cleaned in scalars) {
            return cleaned
        }
        if ('.' in cleaned) {
            val qualified = qualify(pkg, cleaned)
            return if (qualified in index.messages || qualified in index.enums) qualified else cleaned
        }
        for (i in scope.size downTo 0) {
            val candidate = qualify(pkg, qualifyScope(scope.take(i), cleaned))
            if (candidate in index.messages || candidate in index.enums) {
                return candidate
            }
        }
        index.simpleTypes[qualifyScope(scope, cleaned)]?.let { return it }
        index.simpleTypes[cleaned]?.let { return it }
        return qualify(pkg, cleaned)
    }

    private enum class BlockKind {
        SERVICE,
        MESSAGE,
        ENUM,
    }

    private enum class FieldCardinality {
        OPTIONAL,
        REPEATED,
        MAP,
    }

    private data class Block(
        val kind: BlockKind,
        val name: String,
        val service: ServiceDef? = null,
        val message: MessageDef? = null,
        val enumDef: EnumDef? = null,
    )

    private class ProtoIndex {
        val services = mutableListOf<ServiceDef>()
        val messages = linkedMapOf<String, MessageDef>()
        val enums = linkedMapOf<String, EnumDef>()
        val simpleTypes = linkedMapOf<String, String>()
    }

    private data class ServiceDef(
        val fullName: String,
        val comment: CommentMeta,
        val methods: MutableList<MethodDef> = mutableListOf(),
    )

    private data class MethodDef(
        val name: String,
        val inputType: String,
        val outputType: String,
        val clientStreaming: Boolean,
        val serverStreaming: Boolean,
        val comment: CommentMeta,
    )

    private data class MessageDef(
        val fullName: String,
        val pkg: String,
        val scope: List<String>,
        val comment: CommentMeta,
        val fields: MutableList<FieldDef> = mutableListOf(),
    ) {
        fun simpleKey(): String = qualifyScope(scope, fullName.substringAfterLast('.'))
    }

    private data class EnumDef(
        val fullName: String,
        val scope: List<String>,
        val values: MutableList<EnumValueDef> = mutableListOf(),
    ) {
        fun simpleKey(): String = qualifyScope(scope, fullName.substringAfterLast('.'))
    }

    private data class EnumValueDef(
        val name: String,
        val number: Int,
        val comment: CommentMeta,
    )

    private data class FieldDef(
        val name: String,
        val number: Int,
        val comment: CommentMeta,
        val cardinality: FieldCardinality,
        val type: String?,
        val mapKeyType: String?,
        val mapValueType: String?,
        val pkg: String,
        val scope: List<String>,
    ) {
        fun typeName(): String =
            if (cardinality == FieldCardinality.MAP) {
                "map<$mapKeyType, $mapValueType>"
            } else {
                type.orEmpty()
            }

        fun resolvedType(index: ProtoIndex): String = resolveTypeName(type.orEmpty(), pkg, scope, index)

        fun resolvedMapValueType(index: ProtoIndex): String =
            resolveTypeName(mapValueType.orEmpty(), pkg, scope, index)

        fun label(): HolonsDescribe.FieldLabel =
            when (cardinality) {
                FieldCardinality.OPTIONAL -> HolonsDescribe.FieldLabel.FIELD_LABEL_OPTIONAL
                FieldCardinality.REPEATED -> HolonsDescribe.FieldLabel.FIELD_LABEL_REPEATED
                FieldCardinality.MAP -> HolonsDescribe.FieldLabel.FIELD_LABEL_MAP
            }
    }

    private data class CommentMeta(
        val description: String,
        val required: Boolean,
        val example: String,
    ) {
        companion object {
            fun parse(lines: List<String>): CommentMeta {
                val description = mutableListOf<String>()
                val examples = mutableListOf<String>()
                var required = false

                lines.forEach { raw ->
                    val line = raw.trim()
                    when {
                        line.isEmpty() -> Unit
                        line == "@required" -> required = true
                        line.startsWith("@example") -> {
                            val example = line.removePrefix("@example").trim()
                            if (example.isNotEmpty()) {
                                examples += example
                            }
                        }

                        else -> description += line
                    }
                }

                return CommentMeta(
                    description = description.joinToString(" "),
                    required = required,
                    example = examples.joinToString("\n"),
                )
            }
        }
    }
}
