import Foundation

public protocol SettingsStore: AnyObject {
    func readBool(_ key: String, defaultValue: Bool) -> Bool
    func readString(_ key: String, defaultValue: String) -> String
    func writeBool(_ key: String, _ value: Bool)
    func writeString(_ key: String, _ value: String)
}

public final class FileSettingsStore: SettingsStore {
    private let fileURL: URL
    private let lock = NSLock()
    private var values: [String: Any]

    private init(fileURL: URL, values: [String: Any]) {
        self.fileURL = fileURL
        self.values = values
    }

    public static func create(
        applicationId: String = "holons-app",
        applicationName: String = "Holons App"
    ) -> FileSettingsStore {
        let fileManager = FileManager.default
        let directoryURL = settingsDirectoryURL(
            fileManager: fileManager,
            applicationId: applicationId,
            applicationName: applicationName
        )
        try? fileManager.createDirectory(
            at: directoryURL,
            withIntermediateDirectories: true
        )

        let fileURL = directoryURL.appendingPathComponent(
            "settings.json",
            isDirectory: false
        )
        let values = readSettingsFile(fileURL: fileURL)
        return FileSettingsStore(fileURL: fileURL, values: values)
    }

    public func readBool(_ key: String, defaultValue: Bool = false) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return values[key] as? Bool ?? defaultValue
    }

    public func readString(_ key: String, defaultValue: String = "") -> String {
        lock.lock()
        defer { lock.unlock() }
        return values[key] as? String ?? defaultValue
    }

    public func writeBool(_ key: String, _ value: Bool) {
        lock.lock()
        values[key] = value
        let snapshot = values
        lock.unlock()
        flush(snapshot)
    }

    public func writeString(_ key: String, _ value: String) {
        lock.lock()
        values[key] = value
        let snapshot = values
        lock.unlock()
        flush(snapshot)
    }

    private func flush(_ snapshot: [String: Any]) {
        guard JSONSerialization.isValidJSONObject(snapshot),
              let data = try? JSONSerialization.data(
                withJSONObject: snapshot,
                options: [.prettyPrinted, .sortedKeys]
              ),
              var text = String(data: data, encoding: .utf8) else {
            return
        }
        text.append("\n")
        try? text.write(to: fileURL, atomically: true, encoding: .utf8)
    }
}

public final class MemorySettingsStore: SettingsStore {
    private var values: [String: Any] = [:]

    public init() {}

    public func readBool(_ key: String, defaultValue: Bool = false) -> Bool {
        values[key] as? Bool ?? defaultValue
    }

    public func readString(_ key: String, defaultValue: String = "") -> String {
        values[key] as? String ?? defaultValue
    }

    public func writeBool(_ key: String, _ value: Bool) {
        values[key] = value
    }

    public func writeString(_ key: String, _ value: String) {
        values[key] = value
    }
}

final class UserDefaultsSettingsStore: SettingsStore {
    private let defaults: UserDefaults

    init(defaults: UserDefaults) {
        self.defaults = defaults
    }

    func readBool(_ key: String, defaultValue: Bool = false) -> Bool {
        if defaults.object(forKey: key) == nil {
            return defaultValue
        }
        return defaults.bool(forKey: key)
    }

    func readString(_ key: String, defaultValue: String = "") -> String {
        defaults.string(forKey: key) ?? defaultValue
    }

    func writeBool(_ key: String, _ value: Bool) {
        defaults.set(value, forKey: key)
    }

    func writeString(_ key: String, _ value: String) {
        defaults.set(value, forKey: key)
    }
}

private func readSettingsFile(fileURL: URL) -> [String: Any] {
    guard let data = try? Data(contentsOf: fileURL),
          let object = try? JSONSerialization.jsonObject(with: data),
          let dictionary = object as? [String: Any] else {
        return [:]
    }
    return dictionary
}

private func settingsDirectoryURL(
    fileManager: FileManager,
    applicationId: String,
    applicationName: String
) -> URL {
    if let appSupport = fileManager.urls(
        for: .applicationSupportDirectory,
        in: .userDomainMask
    ).first {
        return appSupport
            .appendingPathComponent("Organic Programming", isDirectory: true)
            .appendingPathComponent(applicationName, isDirectory: true)
    }

    return URL(
        fileURLWithPath: fileManager.currentDirectoryPath,
        isDirectory: true
    ).appendingPathComponent(
        ".\(applicationId.trimmingCharacters(in: .whitespacesAndNewlines))",
        isDirectory: true
    )
}
