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
  try resolvedSelection(
    slug,
    availableItems: availableHolons,
    idOf: { $0.slug },
    errorFactory: { GreetingSelectionError.holonNotFound($0) }
  )
}

func validatedTransportSelection(_ value: String) throws -> HolonTransportName {
  try HolonsApp.validatedTransportSelection(
    value,
    errorFactory: { GreetingSelectionError.unsupportedTransport($0) }
  )
}

func resolvedLanguageSelection(
  availableLanguages: [Greeting_V1_Language],
  preferredCode: String
) -> String {
  resolvedPreferredSelection(
    availableItems: availableLanguages,
    preferredID: preferredCode,
    fallbackIDs: ["en"],
    idOf: { $0.code }
  )
}

func validatedLanguageSelection(
  _ value: String,
  availableLanguages: [Greeting_V1_Language]
) throws -> String {
  try validatedSelection(
    value,
    availableItems: availableLanguages,
    idOf: { $0.code },
    errorFactory: { GreetingSelectionError.unsupportedLanguage($0) }
  )
}
