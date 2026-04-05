package org.organicprogramming.gabriel.greeting.kotlincompose.runtime

import holons.v1.Describe.FieldDoc
import holons.v1.Describe.MethodDoc
import holons.v1.Describe.ServiceDoc
import org.organicprogramming.holons.Describe

object DescribeRegistration {
    @Volatile
    private var registered = false

    fun ensureRegistered() {
        if (registered) {
            return
        }

        val protoDir = requireNotNull(AppPaths.findAppProtoDir()) {
            "failed to locate app proto directory for Describe"
        }

        val response = Describe.buildResponse(protoDir)
        if (response.servicesList.none { it.name == "holons.v1.CoaxService" }) {
            response.toBuilder().addServices(coaxServiceDoc()).build().also {
                Describe.useStaticResponse(it)
            }
        } else {
            Describe.useStaticResponse(response)
        }
        registered = true
    }

    private fun coaxServiceDoc(): ServiceDoc =
        ServiceDoc.newBuilder()
            .setName("holons.v1.CoaxService")
            .setDescription(
                "COAX interaction surface for the Gabriel Greeting Kotlin Compose app. It exposes member discovery, connection, and app-level orchestration through the same shared state the UI uses.",
            )
            .addMethods(
                method(
                    name = "ListMembers",
                    description = "List the organism's available member holons.",
                    inputType = "holons.v1.ListMembersRequest",
                    outputType = "holons.v1.ListMembersResponse",
                ),
            )
            .addMethods(
                method(
                    name = "MemberStatus",
                    description = "Query the runtime status of a specific member holon.",
                    inputType = "holons.v1.MemberStatusRequest",
                    outputType = "holons.v1.MemberStatusResponse",
                    inputFields = listOf(
                        field("slug", "string", 1, "Slug of the member holon to inspect."),
                    ),
                ),
            )
            .addMethods(
                method(
                    name = "ConnectMember",
                    description = "Connect a member holon using the app's runtime state.",
                    inputType = "holons.v1.ConnectMemberRequest",
                    outputType = "holons.v1.ConnectMemberResponse",
                    inputFields = listOf(
                        field("slug", "string", 1, "Slug of the member holon to connect."),
                        field("transport", "string", 2, "Optional transport override."),
                    ),
                ),
            )
            .addMethods(
                method(
                    name = "DisconnectMember",
                    description = "Disconnect the currently selected member holon.",
                    inputType = "holons.v1.DisconnectMemberRequest",
                    outputType = "holons.v1.DisconnectMemberResponse",
                ),
            )
            .addMethods(
                method(
                    name = "Tell",
                    description = "Forward an RPC command to a member holon through the organism-level COAX surface.",
                    inputType = "holons.v1.TellRequest",
                    outputType = "holons.v1.TellResponse",
                ),
            )
            .addMethods(
                method(
                    name = "TurnOffCoax",
                    description = "Shut down the COAX server gracefully.",
                    inputType = "holons.v1.TurnOffCoaxRequest",
                    outputType = "holons.v1.TurnOffCoaxResponse",
                ),
            )
            .build()

    private fun method(
        name: String,
        description: String,
        inputType: String,
        outputType: String,
        inputFields: List<FieldDoc> = emptyList(),
    ): MethodDoc =
        MethodDoc.newBuilder()
            .setName(name)
            .setDescription(description)
            .setInputType(inputType)
            .setOutputType(outputType)
            .apply { inputFields.forEach(::addInputFields) }
            .build()

    private fun field(
        name: String,
        type: String,
        number: Int,
        description: String,
    ): FieldDoc =
        FieldDoc.newBuilder()
            .setName(name)
            .setType(type)
            .setNumber(number)
            .setDescription(description)
            .build()
}
