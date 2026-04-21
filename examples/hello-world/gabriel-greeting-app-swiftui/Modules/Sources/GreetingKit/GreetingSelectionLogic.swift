import Foundation
import HolonsApp

enum GreetingSelectionError: LocalizedError {
  case holonNotFound(String)
  case noHolonsFound
  case unsupportedTransport(String)
  case unsupportedLanguage(String)
  case noLanguageSelected

  var errorDescription: String? {
    switch self {
    case .holonNotFound(let slug):
      return "Holon '\(slug)' not found"
    case .noHolonsFound:
      return "No Gabriel holons found"
    case .unsupportedTransport(let value):
      return "Unsupported transport '\(value)'. Expected one of: stdio, tcp, unix"
    case .unsupportedLanguage(let value):
      let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
      if trimmed.isEmpty {
        return "Language code is required"
      }
      return "Unsupported language '\(trimmed)'"
    case .noLanguageSelected:
      return "No language selected"
    }
  }
}

func resolvedHolonSelection(
  slug: String,
  availableHolons: [GabrielHolonIdentity]
) throws -> GabrielHolonIdentity {
  guard let identity = availableHolons.first(where: { $0.slug == slug }) else {
    throw GreetingSelectionError.holonNotFound(slug)
  }
  return identity
}

func validatedTransportSelection(_ value: String) throws -> HolonTransportName {
  guard let transport = HolonTransportName.parseCanonical(value) else {
    throw GreetingSelectionError.unsupportedTransport(value)
  }
  return transport
}

func resolvedLanguageSelection(
  availableLanguages: [Greeting_V1_Language],
  preferredCode: String
) -> String {
  availableLanguages.first(where: { $0.code == preferredCode })?.code
    ?? availableLanguages.first(where: { $0.code == "en" })?.code
    ?? availableLanguages.first?.code
    ?? ""
}

func validatedLanguageSelection(
  _ value: String,
  availableLanguages: [Greeting_V1_Language]
) throws -> String {
  let code = value.trimmingCharacters(in: .whitespacesAndNewlines)
  guard !code.isEmpty else {
    throw GreetingSelectionError.unsupportedLanguage(value)
  }
  guard availableLanguages.contains(where: { $0.code == code }) else {
    throw GreetingSelectionError.unsupportedLanguage(code)
  }
  return code
}
