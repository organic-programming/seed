import Foundation
import Holons
import SwiftProtobuf

enum CoaxDescribeRegistration {
    static func register() throws {
        let payload = try makeCoaxDescribeResponse().serializedData().base64EncodedString()
        try Describe.useStaticResponse(StaticDescribeResponse(payloadBase64: payload))
    }
}

func makeCoaxDescribeResponse() -> Holons_V1_DescribeResponse {
    var response = Holons_V1_DescribeResponse()
    response.manifest = makeManifest()
    response.services = [
        coaxServiceDoc(),
        greetingAppServiceDoc(),
    ]
    return response
}

private func makeManifest() -> Holons_V1_HolonManifest {
    var manifest = Holons_V1_HolonManifest()
    manifest.identity = .with {
        $0.schema = "holon/v1"
        $0.givenName = "Gabriel"
        $0.familyName = "Greeting-App-SwiftUI"
        $0.motto = "SwiftUI HostUI for the Gabriel greeting service."
        $0.composer = "swiftui-example"
        $0.status = "draft"
        $0.born = "2026-03-20"
        if let version = Bundle.main.infoDictionary?["CFBundleVersion"] as? String,
           !version.isEmpty,
           !version.contains("{{")
        {
            $0.version = version
        }
    }
    manifest.lang = "swift"
    manifest.kind = "composite"
    return manifest
}

private func coaxServiceDoc() -> Holons_V1_ServiceDoc {
    var service = Holons_V1_ServiceDoc()
    service.name = "holons.v1.CoaxService"
    service.description_p =
        "COAX interaction surface for the Gabriel Greeting app. It exposes member discovery, connection, and agent-driven UI actions through the same shared state the human interface uses."
    service.methods = [
        methodDoc(
            name: "ListMembers",
            description:
                "List the organism's available member holons. Equivalent to browsing the holon picker in the UI.",
            inputType: "holons.v1.ListMembersRequest",
            outputType: "holons.v1.ListMembersResponse",
            outputFields: [
                fieldDoc(
                    name: "members",
                    type: "holons.v1.MemberInfo",
                    number: 1,
                    description: "Member holons currently available to the app.",
                    label: .repeated,
                    nestedFields: memberInfoFields()
                )
            ],
            exampleInput: "{}"
        ),
        methodDoc(
            name: "MemberStatus",
            description: "Query the runtime status of a specific member holon.",
            inputType: "holons.v1.MemberStatusRequest",
            outputType: "holons.v1.MemberStatusResponse",
            inputFields: [
                fieldDoc(
                    name: "slug",
                    type: "string",
                    number: 1,
                    description: "Slug of the member holon to inspect.",
                    required: true,
                    example: "\"gabriel-greeting-rust\""
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "member",
                    type: "holons.v1.MemberInfo",
                    number: 1,
                    description: "Runtime information for the selected member holon.",
                    nestedFields: memberInfoFields()
                )
            ],
            exampleInput: "{\"slug\":\"gabriel-greeting-rust\"}"
        ),
        methodDoc(
            name: "ConnectMember",
            description:
                "Connect a member holon using the app's runtime state, identical to selecting it in the UI.",
            inputType: "holons.v1.ConnectMemberRequest",
            outputType: "holons.v1.ConnectMemberResponse",
            inputFields: [
                fieldDoc(
                    name: "slug",
                    type: "string",
                    number: 1,
                    description: "Slug of the member holon to connect.",
                    required: true,
                    example: "\"gabriel-greeting-rust\""
                ),
                fieldDoc(
                    name: "transport",
                    type: "string",
                    number: 2,
                    description: "Optional transport override such as stdio, tcp, or unix.",
                    example: "\"stdio\""
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "member",
                    type: "holons.v1.MemberInfo",
                    number: 1,
                    description: "Runtime information for the member after the connection attempt.",
                    nestedFields: memberInfoFields()
                )
            ],
            exampleInput: "{\"slug\":\"gabriel-greeting-rust\",\"transport\":\"stdio\"}"
        ),
        methodDoc(
            name: "DisconnectMember",
            description: "Disconnect the currently selected member holon.",
            inputType: "holons.v1.DisconnectMemberRequest",
            outputType: "holons.v1.DisconnectMemberResponse",
            inputFields: [
                fieldDoc(
                    name: "slug",
                    type: "string",
                    number: 1,
                    description: "Slug of the member holon to disconnect.",
                    required: true,
                    example: "\"gabriel-greeting-rust\""
                )
            ],
            exampleInput: "{\"slug\":\"gabriel-greeting-rust\"}"
        ),
        methodDoc(
            name: "Tell",
            description:
                "Forward an RPC command to a member holon through the organism-level COAX surface.",
            inputType: "holons.v1.TellRequest",
            outputType: "holons.v1.TellResponse",
            inputFields: [
                fieldDoc(
                    name: "member_slug",
                    type: "string",
                    number: 1,
                    description: "Member holon slug to address.",
                    required: true,
                    example: "\"gabriel-greeting-rust\""
                ),
                fieldDoc(
                    name: "method",
                    type: "string",
                    number: 2,
                    description: "Fully qualified RPC method name.",
                    required: true,
                    example: "\"greeting.v1.GreetingService/SayHello\""
                ),
                fieldDoc(
                    name: "payload",
                    type: "bytes",
                    number: 3,
                    description: "JSON request payload encoded as raw bytes.",
                    required: true
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "payload",
                    type: "bytes",
                    number: 1,
                    description: "JSON response payload encoded as raw bytes."
                )
            ]
        ),
        methodDoc(
            name: "TurnOffCoax",
            description: "Shut down the COAX server gracefully.",
            inputType: "holons.v1.TurnOffCoaxRequest",
            outputType: "holons.v1.TurnOffCoaxResponse",
            exampleInput: "{}"
        ),
    ]
    return service
}

private func greetingAppServiceDoc() -> Holons_V1_ServiceDoc {
    var service = Holons_V1_ServiceDoc()
    service.name = "greeting.v1.GreetingAppService"
    service.description_p =
        "High-level domain RPCs for the Gabriel Greeting UI. Agents use these methods to drive the same selections and greeting flow a human performs in the app."
    service.methods = [
        methodDoc(
            name: "SelectHolon",
            description: "Select which greeting holon the UI should use.",
            inputType: "greeting.v1.SelectHolonRequest",
            outputType: "greeting.v1.SelectHolonResponse",
            inputFields: [
                fieldDoc(
                    name: "slug",
                    type: "string",
                    number: 1,
                    description: "Slug of the greeting holon to select.",
                    required: true,
                    example: "\"gabriel-greeting-rust\""
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "slug",
                    type: "string",
                    number: 1,
                    description: "Slug of the newly selected holon."
                ),
                fieldDoc(
                    name: "display_name",
                    type: "string",
                    number: 2,
                    description: "Human-readable display name shown in the UI."
                )
            ],
            exampleInput: "{\"slug\":\"gabriel-greeting-rust\"}"
        ),
        methodDoc(
            name: "SelectTransport",
            description: "Select which transport the greeting holon connection should use.",
            inputType: "greeting.v1.SelectTransportRequest",
            outputType: "greeting.v1.SelectTransportResponse",
            inputFields: [
                fieldDoc(
                    name: "transport",
                    type: "string",
                    number: 1,
                    description: "Canonical transport name: stdio, tcp, or unix.",
                    required: true,
                    example: "\"tcp\""
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "transport",
                    type: "string",
                    number: 1,
                    description: "The canonical transport now selected by the UI."
                )
            ],
            exampleInput: "{\"transport\":\"tcp\"}"
        ),
        methodDoc(
            name: "SelectLanguage",
            description: "Select the language used by the greeting UI.",
            inputType: "greeting.v1.SelectLanguageRequest",
            outputType: "greeting.v1.SelectLanguageResponse",
            inputFields: [
                fieldDoc(
                    name: "code",
                    type: "string",
                    number: 1,
                    description: "ISO 639-1 language code to select.",
                    required: true,
                    example: "\"fr\""
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "code",
                    type: "string",
                    number: 1,
                    description: "The selected ISO 639-1 language code."
                )
            ],
            exampleInput: "{\"code\":\"fr\"}"
        ),
        methodDoc(
            name: "Greet",
            description: "Produce a greeting using the current holon and language selection.",
            inputType: "greeting.v1.GreetRequest",
            outputType: "greeting.v1.GreetResponse",
            inputFields: [
                fieldDoc(
                    name: "name",
                    type: "string",
                    number: 1,
                    description: "Name to greet.",
                    example: "\"Maria\""
                ),
                fieldDoc(
                    name: "lang_code",
                    type: "string",
                    number: 2,
                    description: "Optional language override. If empty, uses the UI's current language selection.",
                    example: "\"en\""
                )
            ],
            outputFields: [
                fieldDoc(
                    name: "greeting",
                    type: "string",
                    number: 1,
                    description: "Localized greeting text returned by the selected holon."
                )
            ],
            exampleInput: "{\"name\":\"Maria\",\"lang_code\":\"en\"}"
        ),
    ]
    return service
}

private func memberInfoFields() -> [Holons_V1_FieldDoc] {
    [
        fieldDoc(
            name: "slug",
            type: "string",
            number: 1,
            description: "The member holon's slug."
        ),
        fieldDoc(
            name: "identity",
            type: "holons.v1.HolonManifest.Identity",
            number: 2,
            description: "Identity information exposed by the member holon.",
            nestedFields: identityFields()
        ),
        fieldDoc(
            name: "state",
            type: "holons.v1.MemberState",
            number: 3,
            description: "Current runtime state for the member holon.",
            enumValues: memberStateValues()
        ),
        fieldDoc(
            name: "is_organism",
            type: "bool",
            number: 4,
            description: "Whether this member is itself an organism exposing COAX."
        ),
    ]
}

private func identityFields() -> [Holons_V1_FieldDoc] {
    [
        fieldDoc(
            name: "given_name",
            type: "string",
            number: 3,
            description: "Given name from the member holon's identity.",
            example: "\"Gabriel\""
        ),
        fieldDoc(
            name: "family_name",
            type: "string",
            number: 4,
            description: "Family name from the member holon's identity.",
            example: "\"Greeting-Rust\""
        ),
        fieldDoc(
            name: "motto",
            type: "string",
            number: 5,
            description: "Short descriptive motto for the member holon."
        ),
    ]
}

private func memberStateValues() -> [Holons_V1_EnumValueDoc] {
    [
        enumValueDoc(
            name: "MEMBER_STATE_UNSPECIFIED",
            number: 0,
            description: "No member state has been established yet."
        ),
        enumValueDoc(
            name: "MEMBER_STATE_AVAILABLE",
            number: 1,
            description: "Known to the app but not running."
        ),
        enumValueDoc(
            name: "MEMBER_STATE_CONNECTING",
            number: 2,
            description: "Process starting, not yet ready."
        ),
        enumValueDoc(
            name: "MEMBER_STATE_CONNECTED",
            number: 3,
            description: "Connected and ready for RPC."
        ),
        enumValueDoc(
            name: "MEMBER_STATE_ERROR",
            number: 4,
            description: "Connection or process error."
        ),
    ]
}

private func methodDoc(
    name: String,
    description: String,
    inputType: String,
    outputType: String,
    inputFields: [Holons_V1_FieldDoc] = [],
    outputFields: [Holons_V1_FieldDoc] = [],
    exampleInput: String = ""
) -> Holons_V1_MethodDoc {
    var method = Holons_V1_MethodDoc()
    method.name = name
    method.description_p = description
    method.inputType = inputType
    method.outputType = outputType
    method.inputFields = inputFields
    method.outputFields = outputFields
    method.exampleInput = exampleInput
    return method
}

private func fieldDoc(
    name: String,
    type: String,
    number: Int32,
    description: String,
    label: Holons_V1_FieldLabel = .optional,
    required: Bool = false,
    example: String = "",
    nestedFields: [Holons_V1_FieldDoc] = [],
    enumValues: [Holons_V1_EnumValueDoc] = []
) -> Holons_V1_FieldDoc {
    var field = Holons_V1_FieldDoc()
    field.name = name
    field.type = type
    field.number = number
    field.description_p = description
    field.label = label
    field.required = required
    field.example = example
    field.nestedFields = nestedFields
    field.enumValues = enumValues
    return field
}

private func enumValueDoc(
    name: String,
    number: Int32,
    description: String
) -> Holons_V1_EnumValueDoc {
    var value = Holons_V1_EnumValueDoc()
    value.name = name
    value.number = number
    value.description_p = description
    return value
}
