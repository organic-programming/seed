import Foundation
import SwiftProtobuf

private let _protobuf_package = "greeting.v1"

struct Greeting_V1_ListLanguagesRequest: Sendable {
    var unknownFields = SwiftProtobuf.UnknownStorage()
    init() {}
}

struct Greeting_V1_ListLanguagesResponse: Sendable {
    var languages: [Greeting_V1_Language] = []
    var unknownFields = SwiftProtobuf.UnknownStorage()
    init() {}
}

struct Greeting_V1_Language: Sendable {
    var code: String = ""
    var name: String = ""
    var native_p: String = ""
    var unknownFields = SwiftProtobuf.UnknownStorage()
    init() {}
}

struct Greeting_V1_SayHelloRequest: Sendable {
    var name: String = ""
    var langCode: String = ""
    var unknownFields = SwiftProtobuf.UnknownStorage()
    init() {}
}

struct Greeting_V1_SayHelloResponse: Sendable {
    var greeting: String = ""
    var language: String = ""
    var langCode: String = ""
    var unknownFields = SwiftProtobuf.UnknownStorage()
    init() {}
}

extension Greeting_V1_ListLanguagesRequest: SwiftProtobuf.Message, SwiftProtobuf._MessageImplementationBase, SwiftProtobuf._ProtoNameProviding {
    static let protoMessageName = _protobuf_package + ".ListLanguagesRequest"
    static let _protobuf_nameMap = SwiftProtobuf._NameMap(bytecode: "")

    mutating func decodeMessage<D: SwiftProtobuf.Decoder>(decoder: inout D) throws {
        while let _ = try decoder.nextFieldNumber() {}
    }

    func traverse<V: SwiftProtobuf.Visitor>(visitor: inout V) throws {
        try unknownFields.traverse(visitor: &visitor)
    }

    static func == (lhs: Greeting_V1_ListLanguagesRequest, rhs: Greeting_V1_ListLanguagesRequest) -> Bool {
        lhs.unknownFields == rhs.unknownFields
    }
}

extension Greeting_V1_ListLanguagesResponse: SwiftProtobuf.Message, SwiftProtobuf._MessageImplementationBase, SwiftProtobuf._ProtoNameProviding {
    static let protoMessageName = _protobuf_package + ".ListLanguagesResponse"
    static let _protobuf_nameMap = SwiftProtobuf._NameMap(bytecode: "\u{0}\u{2}\u{2}languages\u{0}\u{c}\u{1}\u{1}")

    mutating func decodeMessage<D: SwiftProtobuf.Decoder>(decoder: inout D) throws {
        while let fieldNumber = try decoder.nextFieldNumber() {
            switch fieldNumber {
            case 1: try { try decoder.decodeRepeatedMessageField(value: &languages) }()
            default: break
            }
        }
    }

    func traverse<V: SwiftProtobuf.Visitor>(visitor: inout V) throws {
        if !languages.isEmpty {
            try visitor.visitRepeatedMessageField(value: languages, fieldNumber: 1)
        }
        try unknownFields.traverse(visitor: &visitor)
    }

    static func == (lhs: Greeting_V1_ListLanguagesResponse, rhs: Greeting_V1_ListLanguagesResponse) -> Bool {
        lhs.languages == rhs.languages && lhs.unknownFields == rhs.unknownFields
    }
}

extension Greeting_V1_Language: SwiftProtobuf.Message, SwiftProtobuf._MessageImplementationBase, SwiftProtobuf._ProtoNameProviding {
    static let protoMessageName = _protobuf_package + ".Language"
    static let _protobuf_nameMap = SwiftProtobuf._NameMap(bytecode: "\u{0}\u{3}code\u{0}\u{3}name\u{0}\u{3}native\u{0}")

    mutating func decodeMessage<D: SwiftProtobuf.Decoder>(decoder: inout D) throws {
        while let fieldNumber = try decoder.nextFieldNumber() {
            switch fieldNumber {
            case 1: try { try decoder.decodeSingularStringField(value: &code) }()
            case 2: try { try decoder.decodeSingularStringField(value: &name) }()
            case 3: try { try decoder.decodeSingularStringField(value: &native_p) }()
            default: break
            }
        }
    }

    func traverse<V: SwiftProtobuf.Visitor>(visitor: inout V) throws {
        if !code.isEmpty { try visitor.visitSingularStringField(value: code, fieldNumber: 1) }
        if !name.isEmpty { try visitor.visitSingularStringField(value: name, fieldNumber: 2) }
        if !native_p.isEmpty { try visitor.visitSingularStringField(value: native_p, fieldNumber: 3) }
        try unknownFields.traverse(visitor: &visitor)
    }

    static func == (lhs: Greeting_V1_Language, rhs: Greeting_V1_Language) -> Bool {
        lhs.code == rhs.code &&
        lhs.name == rhs.name &&
        lhs.native_p == rhs.native_p &&
        lhs.unknownFields == rhs.unknownFields
    }
}

extension Greeting_V1_SayHelloRequest: SwiftProtobuf.Message, SwiftProtobuf._MessageImplementationBase, SwiftProtobuf._ProtoNameProviding {
    static let protoMessageName = _protobuf_package + ".SayHelloRequest"
    static let _protobuf_nameMap = SwiftProtobuf._NameMap(bytecode: "\u{0}\u{3}name\u{0}\u{3}lang_code\u{0}")

    mutating func decodeMessage<D: SwiftProtobuf.Decoder>(decoder: inout D) throws {
        while let fieldNumber = try decoder.nextFieldNumber() {
            switch fieldNumber {
            case 1: try { try decoder.decodeSingularStringField(value: &name) }()
            case 2: try { try decoder.decodeSingularStringField(value: &langCode) }()
            default: break
            }
        }
    }

    func traverse<V: SwiftProtobuf.Visitor>(visitor: inout V) throws {
        if !name.isEmpty { try visitor.visitSingularStringField(value: name, fieldNumber: 1) }
        if !langCode.isEmpty { try visitor.visitSingularStringField(value: langCode, fieldNumber: 2) }
        try unknownFields.traverse(visitor: &visitor)
    }

    static func == (lhs: Greeting_V1_SayHelloRequest, rhs: Greeting_V1_SayHelloRequest) -> Bool {
        lhs.name == rhs.name &&
        lhs.langCode == rhs.langCode &&
        lhs.unknownFields == rhs.unknownFields
    }
}

extension Greeting_V1_SayHelloResponse: SwiftProtobuf.Message, SwiftProtobuf._MessageImplementationBase, SwiftProtobuf._ProtoNameProviding {
    static let protoMessageName = _protobuf_package + ".SayHelloResponse"
    static let _protobuf_nameMap = SwiftProtobuf._NameMap(bytecode: "\u{0}\u{3}greeting\u{0}\u{3}language\u{0}\u{3}lang_code\u{0}")

    mutating func decodeMessage<D: SwiftProtobuf.Decoder>(decoder: inout D) throws {
        while let fieldNumber = try decoder.nextFieldNumber() {
            switch fieldNumber {
            case 1: try { try decoder.decodeSingularStringField(value: &greeting) }()
            case 2: try { try decoder.decodeSingularStringField(value: &language) }()
            case 3: try { try decoder.decodeSingularStringField(value: &langCode) }()
            default: break
            }
        }
    }

    func traverse<V: SwiftProtobuf.Visitor>(visitor: inout V) throws {
        if !greeting.isEmpty { try visitor.visitSingularStringField(value: greeting, fieldNumber: 1) }
        if !language.isEmpty { try visitor.visitSingularStringField(value: language, fieldNumber: 2) }
        if !langCode.isEmpty { try visitor.visitSingularStringField(value: langCode, fieldNumber: 3) }
        try unknownFields.traverse(visitor: &visitor)
    }

    static func == (lhs: Greeting_V1_SayHelloResponse, rhs: Greeting_V1_SayHelloResponse) -> Bool {
        lhs.greeting == rhs.greeting &&
        lhs.language == rhs.language &&
        lhs.langCode == rhs.langCode &&
        lhs.unknownFields == rhs.unknownFields
    }
}
