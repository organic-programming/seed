import Foundation

public enum HolonTransportName: String, CaseIterable, Identifiable, Sendable {
    case stdio
    case tcp
    case unix

    public var id: String { rawValue }

    public var title: String { rawValue }

    public static func parseCanonical(_ value: String) -> HolonTransportName? {
        switch value.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case HolonTransportName.stdio.rawValue:
            return .stdio
        case HolonTransportName.tcp.rawValue:
            return .tcp
        case HolonTransportName.unix.rawValue:
            return .unix
        default:
            return nil
        }
    }

    public static func normalize(_ value: String?) -> HolonTransportName {
        switch value?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case nil, "", "auto", "stdio", "stdio://":
            return .stdio
        case "tcp", "tcp://":
            return .tcp
        case "unix", "unix://":
            return .unix
        default:
            return .stdio
        }
    }

    @available(*, deprecated, renamed: "parseCanonical")
    public static func validatedRPCName(_ value: String) -> HolonTransportName? {
        parseCanonical(value)
    }

    @available(*, deprecated, renamed: "normalize")
    public static func normalizedSelection(_ value: String?) -> HolonTransportName {
        normalize(value)
    }
}

@available(*, deprecated, renamed: "HolonTransportName")
public typealias GreetingTransportName = HolonTransportName
