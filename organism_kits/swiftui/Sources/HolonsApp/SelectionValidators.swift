import Foundation

public enum SelectionValidationError: LocalizedError, Equatable {
    case emptySelection
    case itemNotFound(String)
    case unsupportedTransport(String)

    public var errorDescription: String? {
        switch self {
        case .emptySelection:
            return "Selection is required"
        case .itemNotFound(let id):
            return "Selection '\(id)' was not found"
        case .unsupportedTransport(let value):
            return "Unsupported transport '\(value)'. Expected one of: stdio, tcp, unix"
        }
    }
}

public func resolvedSelection<T>(
    _ id: String,
    availableItems: [T],
    idOf: (T) -> String,
    errorFactory: (String) -> Error = { SelectionValidationError.itemNotFound($0) }
) throws -> T {
    guard let item = availableItems.first(where: { idOf($0) == id }) else {
        throw errorFactory(id)
    }
    return item
}

public func resolvedPreferredSelection<T>(
    availableItems: [T],
    preferredID: String,
    fallbackIDs: [String] = [],
    idOf: (T) -> String
) -> String {
    if let preferred = availableItems.first(where: { idOf($0) == preferredID }) {
        return idOf(preferred)
    }
    for fallbackID in fallbackIDs {
        if let fallback = availableItems.first(where: { idOf($0) == fallbackID }) {
            return idOf(fallback)
        }
    }
    return availableItems.first.map(idOf) ?? ""
}

public func validatedSelection<T>(
    _ value: String,
    availableItems: [T],
    idOf: (T) -> String,
    errorFactory: (String) -> Error = { SelectionValidationError.itemNotFound($0) }
) throws -> String {
    let id = value.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !id.isEmpty else {
        throw errorFactory(value)
    }
    _ = try resolvedSelection(id, availableItems: availableItems, idOf: idOf, errorFactory: errorFactory)
    return id
}

public func validatedTransportSelection(
    _ value: String,
    errorFactory: (String) -> Error = { SelectionValidationError.unsupportedTransport($0) }
) throws -> HolonTransportName {
    guard let transport = HolonTransportName.parseCanonical(value) else {
        throw errorFactory(value)
    }
    return transport
}
