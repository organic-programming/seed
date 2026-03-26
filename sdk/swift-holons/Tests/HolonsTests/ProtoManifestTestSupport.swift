import Foundation

func writeHolonProto(
  to url: URL,
  packageName: String = "test.v1",
  uuid: String,
  givenName: String,
  familyName: String,
  motto: String = "",
  composer: String = "test",
  clade: String? = nil,
  status: String = "draft",
  born: String = "2026-03-17",
  lang: String? = nil,
  kind: String? = nil,
  buildRunner: String? = nil,
  buildMain: String? = nil,
  artifactBinary: String? = nil,
  artifactPrimary: String? = nil,
  parents: [String] = [],
  reproduction: String? = nil,
  generatedBy: String? = nil,
  aliases: [String] = []
) throws {
  var identityLines = [
    #"    uuid: "\#(uuid)""#,
    #"    given_name: "\#(givenName)""#,
    #"    family_name: "\#(familyName)""#,
  ]
  if !motto.isEmpty {
    identityLines.append(#"    motto: "\#(motto)""#)
  }
  if !composer.isEmpty {
    identityLines.append(#"    composer: "\#(composer)""#)
  }
  if let clade, !clade.isEmpty {
    identityLines.append(#"    clade: "\#(clade)""#)
  }
  if !status.isEmpty {
    identityLines.append(#"    status: "\#(status)""#)
  }
  if !born.isEmpty {
    identityLines.append(#"    born: "\#(born)""#)
  }
  if !aliases.isEmpty {
    identityLines.append("    aliases: \(protoStringArray(aliases))")
  }

  var manifestLines = [
    #"option (holons.v1.manifest) = {"#,
    "  identity: {",
    identityLines.joined(separator: "\n"),
    "  }",
  ]

  if !parents.isEmpty || reproduction != nil || generatedBy != nil {
    var lineageLines: [String] = []
    if !parents.isEmpty {
      lineageLines.append("    parents: \(protoStringArray(parents))")
    }
    if let reproduction, !reproduction.isEmpty {
      lineageLines.append(#"    reproduction: "\#(reproduction)""#)
    }
    if let generatedBy, !generatedBy.isEmpty {
      lineageLines.append(#"    generated_by: "\#(generatedBy)""#)
    }
    manifestLines.append("  lineage: {")
    manifestLines.append(lineageLines.joined(separator: "\n"))
    manifestLines.append("  }")
  }

  if let lang, !lang.isEmpty {
    manifestLines.append(#"  lang: "\#(lang)""#)
  }
  if let kind, !kind.isEmpty {
    manifestLines.append(#"  kind: "\#(kind)""#)
  }
  if buildRunner != nil || buildMain != nil {
    var buildLines: [String] = []
    if let buildRunner, !buildRunner.isEmpty {
      buildLines.append(#"    runner: "\#(buildRunner)""#)
    }
    if let buildMain, !buildMain.isEmpty {
      buildLines.append(#"    main: "\#(buildMain)""#)
    }
    manifestLines.append("  build: {")
    manifestLines.append(buildLines.joined(separator: "\n"))
    manifestLines.append("  }")
  }
  if artifactBinary != nil || artifactPrimary != nil {
    var artifactLines: [String] = []
    if let artifactBinary, !artifactBinary.isEmpty {
      artifactLines.append(#"    binary: "\#(artifactBinary)""#)
    }
    if let artifactPrimary, !artifactPrimary.isEmpty {
      artifactLines.append(#"    primary: "\#(artifactPrimary)""#)
    }
    manifestLines.append("  artifacts: {")
    manifestLines.append(artifactLines.joined(separator: "\n"))
    manifestLines.append("  }")
  }
  manifestLines.append("};")

  let proto = [
    #"syntax = "proto3";"#,
    "",
    "package \(packageName);",
    "",
    #"import "holons/v1/manifest.proto";"#,
    "",
    manifestLines.joined(separator: "\n"),
    "",
  ].joined(separator: "\n")

  try FileManager.default.createDirectory(
    at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
  try proto.write(to: url, atomically: true, encoding: .utf8)
}

private func protoStringArray(_ values: [String]) -> String {
  "[" + values.map { #""\#($0)""# }.joined(separator: ", ") + "]"
}
