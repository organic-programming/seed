import Foundation

public enum GreetingTransportName: String, CaseIterable, Identifiable, Sendable {
    case stdio
    case tcp
    case unix

    public var id: String { rawValue }

    public static func validatedRPCName(_ value: String) -> GreetingTransportName? {
        switch value.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case GreetingTransportName.stdio.rawValue:
            return .stdio
        case GreetingTransportName.tcp.rawValue:
            return .tcp
        case GreetingTransportName.unix.rawValue:
            return .unix
        default:
            return nil
        }
    }

    public static func normalizedSelection(_ value: String?) -> GreetingTransportName {
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
}
