#if os(macOS)
import Foundation
import Holons

public protocol Holons<Holon> {
    associatedtype Holon
    func list() throws -> [Holon]
}

public final class BundledHolons<Holon>: Holons {
    private let fromDiscovered: (HolonEntry) -> Holon?
    private let slugOf: (Holon) -> String
    private let sortRankOf: ((Holon) -> Int)?
    private let displayNameOf: ((Holon) -> String)?

    public init(
        fromDiscovered: @escaping (HolonEntry) -> Holon?,
        slugOf: @escaping (Holon) -> String,
        sortRankOf: ((Holon) -> Int)? = nil,
        displayNameOf: ((Holon) -> String)? = nil
    ) {
        self.fromDiscovered = fromDiscovered
        self.slugOf = slugOf
        self.sortRankOf = sortRankOf
        self.displayNameOf = displayNameOf
    }

    public func list() throws -> [Holon] {
        var deduped: [String: Holon] = [:]
        for entry in try discoverAll() {
            guard let holon = fromDiscovered(entry) else {
                continue
            }
            deduped[slugOf(holon)] = deduped[slugOf(holon)] ?? holon
        }

        return deduped.values.sorted { left, right in
            let leftRank = sortRankOf?(left) ?? 999
            let rightRank = sortRankOf?(right) ?? 999
            if leftRank != rightRank {
                return leftRank < rightRank
            }
            let leftTitle = displayNameOf?(left) ?? slugOf(left)
            let rightTitle = displayNameOf?(right) ?? slugOf(right)
            return leftTitle.localizedCaseInsensitiveCompare(rightTitle) == .orderedAscending
        }
    }
}
#endif
