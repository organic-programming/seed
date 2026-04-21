//! Parse holon.proto identity files.

use regex::Regex;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::OnceLock;

pub const PROTO_MANIFEST_FILE_NAME: &str = "holon.proto";

/// Parsed identity from a holon.proto file.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct HolonIdentity {
    pub uuid: String,
    pub given_name: String,
    pub family_name: String,
    pub motto: String,
    pub composer: String,
    pub clade: String,
    pub status: String,
    pub born: String,
    pub version: String,
    pub lang: String,
    pub parents: Vec<String>,
    pub reproduction: String,
    pub generated_by: String,
    pub proto_status: String,
    pub aliases: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResolvedManifest {
    pub identity: HolonIdentity,
    pub description: String,
    pub kind: String,
    pub build_runner: String,
    pub build_main: String,
    pub artifact_binary: String,
    pub artifact_primary: String,
    pub required_files: Vec<String>,
    pub member_paths: Vec<String>,
    pub skills: Vec<ResolvedSkill>,
    pub sequences: Vec<ResolvedSequence>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResolvedSkill {
    pub name: String,
    pub description: String,
    pub when: String,
    pub steps: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResolvedSequence {
    pub name: String,
    pub description: String,
    pub params: Vec<ResolvedSequenceParam>,
    pub steps: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResolvedSequenceParam {
    pub name: String,
    pub description: String,
    pub required: bool,
    pub default: String,
}

impl HolonIdentity {
    /// Return the canonical slug derived from the holon's identity.
    pub fn slug(&self) -> String {
        let given = self.given_name.trim();
        let family = self.family_name.trim().trim_end_matches('?');
        if given.is_empty() && family.is_empty() {
            return String::new();
        }

        format!("{given}-{family}")
            .trim()
            .to_lowercase()
            .replace(' ', "-")
            .trim_matches('-')
            .to_string()
    }
}

/// Parse a holon.proto file and return its identity.
pub fn parse_holon(path: &Path) -> Result<HolonIdentity, Box<dyn std::error::Error>> {
    Ok(resolve_proto_file(path)?.identity)
}

pub fn resolve(root: &Path) -> Result<ResolvedManifest, Box<dyn std::error::Error>> {
    let manifest_path = resolve_manifest_path(root)?;
    resolve_proto_file(&manifest_path)
}

/// Parse manifest fields from a holon.proto file.
pub fn parse_manifest(path: &Path) -> Result<ResolvedManifest, Box<dyn std::error::Error>> {
    resolve_proto_file(path)
}

pub fn resolve_proto_file(path: &Path) -> Result<ResolvedManifest, Box<dyn std::error::Error>> {
    if path.file_name().and_then(|name| name.to_str()) != Some(PROTO_MANIFEST_FILE_NAME) {
        return Err(format!(
            "{} is not a {} file",
            path.display(),
            PROTO_MANIFEST_FILE_NAME
        )
        .into());
    }

    let text = fs::read_to_string(path)?;
    let manifest_block = extract_manifest_block(&text).ok_or_else(|| {
        format!(
            "{}: missing holons.v1.manifest option in holon.proto",
            path.display()
        )
    })?;

    let identity_block = extract_block("identity", &manifest_block).unwrap_or_default();
    let lineage_block = extract_block("lineage", &manifest_block).unwrap_or_default();
    let build_block = extract_block("build", &manifest_block).unwrap_or_default();
    let requires_block = extract_block("requires", &manifest_block).unwrap_or_default();
    let artifacts_block = extract_block("artifacts", &manifest_block).unwrap_or_default();
    let member_paths = extract_blocks("members", &build_block)
        .into_iter()
        .map(|member| scalar("path", &member))
        .collect::<Vec<_>>();
    let skills = extract_blocks("skills", &manifest_block)
        .into_iter()
        .map(|skill| ResolvedSkill {
            name: scalar("name", &skill),
            description: scalar("description", &skill),
            when: scalar("when", &skill),
            steps: compact_strings(string_list("steps", &skill)),
        })
        .collect::<Vec<_>>();
    let sequences = extract_blocks("sequences", &manifest_block)
        .into_iter()
        .map(|sequence| ResolvedSequence {
            name: scalar("name", &sequence),
            description: scalar("description", &sequence),
            params: extract_blocks("params", &sequence)
                .into_iter()
                .map(|param| ResolvedSequenceParam {
                    name: scalar("name", &param),
                    description: scalar("description", &param),
                    required: boolean("required", &param),
                    default: scalar("default", &param),
                })
                .collect(),
            steps: compact_strings(string_list("steps", &sequence)),
        })
        .collect::<Vec<_>>();

    Ok(ResolvedManifest {
        identity: HolonIdentity {
            uuid: scalar("uuid", &identity_block),
            given_name: scalar("given_name", &identity_block),
            family_name: scalar("family_name", &identity_block),
            motto: scalar("motto", &identity_block),
            composer: scalar("composer", &identity_block),
            clade: scalar("clade", &identity_block),
            status: scalar("status", &identity_block),
            born: scalar("born", &identity_block),
            version: scalar("version", &identity_block),
            lang: scalar("lang", &manifest_block),
            parents: string_list("parents", &lineage_block),
            reproduction: scalar("reproduction", &lineage_block),
            generated_by: scalar("generated_by", &lineage_block),
            proto_status: scalar("proto_status", &identity_block),
            aliases: string_list("aliases", &identity_block),
        },
        description: scalar("description", &manifest_block),
        kind: scalar("kind", &manifest_block),
        build_runner: scalar("runner", &build_block),
        build_main: scalar("main", &build_block),
        artifact_binary: scalar("binary", &artifacts_block),
        artifact_primary: scalar("primary", &artifacts_block),
        required_files: compact_strings(string_list("files", &requires_block)),
        member_paths: compact_strings(member_paths),
        skills,
        sequences,
    })
}

pub fn find_holon_proto(root: &Path) -> Option<PathBuf> {
    if root.is_file() {
        return if root.file_name().and_then(|name| name.to_str()) == Some(PROTO_MANIFEST_FILE_NAME)
        {
            Some(root.to_path_buf())
        } else {
            None
        };
    }
    if !root.is_dir() {
        return None;
    }

    let direct = root.join(PROTO_MANIFEST_FILE_NAME);
    if direct.is_file() {
        return Some(direct);
    }

    let api_v1 = root.join("api").join("v1").join(PROTO_MANIFEST_FILE_NAME);
    if api_v1.is_file() {
        return Some(api_v1);
    }

    let mut candidates = Vec::new();
    walk_for_holon_proto(root, &mut candidates);
    candidates.sort();
    candidates.into_iter().next()
}

pub fn resolve_manifest_path(root: &Path) -> Result<PathBuf, Box<dyn std::error::Error>> {
    let normalized = root.to_path_buf();
    let mut search_roots = vec![normalized.clone()];
    if normalized.file_name().and_then(|name| name.to_str()) == Some("protos") {
        if let Some(parent) = normalized.parent() {
            search_roots.push(parent.to_path_buf());
        }
    } else if let Some(parent) = normalized.parent() {
        if parent != normalized {
            search_roots.push(parent.to_path_buf());
        }
    }

    for candidate_root in search_roots {
        if let Some(candidate) = find_holon_proto(&candidate_root) {
            return Ok(candidate);
        }
    }

    Err(format!("no holon.proto found near {}", root.display()).into())
}

fn walk_for_holon_proto(dir: &Path, out: &mut Vec<PathBuf>) {
    let entries = match fs::read_dir(dir) {
        Ok(entries) => entries,
        Err(_) => return,
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            walk_for_holon_proto(&path, out);
            continue;
        }
        if path.file_name().and_then(|name| name.to_str()) == Some(PROTO_MANIFEST_FILE_NAME) {
            out.push(path);
        }
    }
}

fn extract_manifest_block(source: &str) -> Option<String> {
    static RE: OnceLock<Regex> = OnceLock::new();
    let regex =
        RE.get_or_init(|| Regex::new(r"option\s*\(\s*holons\.v1\.manifest\s*\)\s*=\s*\{").unwrap());
    let matched = regex.find(source)?;
    let brace_index = source[matched.start()..].find('{')? + matched.start();
    balanced_block_contents(source, brace_index)
}

fn extract_block(name: &str, source: &str) -> Option<String> {
    let regex = Regex::new(&format!(r"\b{}\s*:\s*\{{", regex::escape(name))).unwrap();
    let matched = regex.find(source)?;
    let brace_index = source[matched.start()..].find('{')? + matched.start();
    balanced_block_contents(source, brace_index)
}

fn scalar(name: &str, source: &str) -> String {
    let quoted = Regex::new(&format!(
        r#"\b{}\s*:\s*"((?:[^"\\]|\\.)*)""#,
        regex::escape(name)
    ))
    .unwrap();
    if let Some(captures) = quoted.captures(source) {
        return unescape_proto_string(captures.get(1).map(|m| m.as_str()).unwrap_or_default());
    }

    let bare = Regex::new(&format!(r"\b{}\s*:\s*([^\s,\]\}}]+)", regex::escape(name))).unwrap();
    bare.captures(source)
        .and_then(|captures| captures.get(1).map(|m| m.as_str().to_string()))
        .unwrap_or_default()
}

fn boolean(name: &str, source: &str) -> bool {
    matches!(scalar(name, source).trim(), "true" | "True" | "TRUE" | "1")
}

fn string_list(name: &str, source: &str) -> Vec<String> {
    let regex = Regex::new(&format!(r"(?s)\b{}\s*:\s*\[(.*?)\]", regex::escape(name))).unwrap();
    let Some(body) = regex
        .captures(source)
        .and_then(|captures| captures.get(1).map(|m| m.as_str().to_string()))
    else {
        return Vec::new();
    };

    let token = Regex::new(r#""((?:[^"\\]|\\.)*)"|([^\s,\]]+)"#).unwrap();
    token
        .captures_iter(&body)
        .filter_map(|captures| {
            captures
                .get(1)
                .map(|m| unescape_proto_string(m.as_str()))
                .or_else(|| captures.get(2).map(|m| m.as_str().to_string()))
        })
        .collect()
}

fn extract_blocks(name: &str, source: &str) -> Vec<String> {
    let regex = Regex::new(&format!(r"\b{}\s*:\s*\{{", regex::escape(name))).unwrap();
    let mut blocks = Vec::new();
    let mut offset = 0;

    while let Some(matched) = regex.find(&source[offset..]) {
        let start = offset + matched.start();
        let Some(brace_index) = source[start..].find('{').map(|index| start + index) else {
            break;
        };
        let Some(block) = balanced_block_contents(source, brace_index) else {
            break;
        };
        offset = brace_index + block.len() + 2;
        blocks.push(block);
    }

    blocks
}

fn compact_strings(values: Vec<String>) -> Vec<String> {
    values
        .into_iter()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .collect()
}

fn balanced_block_contents(source: &str, opening_brace: usize) -> Option<String> {
    let mut depth = 0;
    let mut inside_string = false;
    let mut escaped = false;
    let content_start = opening_brace + 1;

    for (index, ch) in source
        .char_indices()
        .skip_while(|(index, _)| *index < opening_brace)
    {
        if inside_string {
            if escaped {
                escaped = false;
            } else if ch == '\\' {
                escaped = true;
            } else if ch == '"' {
                inside_string = false;
            }
            continue;
        }

        if ch == '"' {
            inside_string = true;
        } else if ch == '{' {
            depth += 1;
        } else if ch == '}' {
            depth -= 1;
            if depth == 0 {
                return Some(source[content_start..index].to_string());
            }
        }
    }

    None
}

fn unescape_proto_string(value: &str) -> String {
    value.replace("\\\"", "\"").replace("\\\\", "\\")
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;

    #[test]
    fn test_parse_holon() {
        let dir = std::env::temp_dir();
        let path = dir.join("holon.proto");
        let mut f = fs::File::create(&path).unwrap();
        writeln!(
            f,
            "syntax = \"proto3\";\n\npackage test.v1;\n\noption (holons.v1.manifest) = {{\n  identity: {{\n    uuid: \"abc-123\"\n    given_name: \"test\"\n    family_name: \"Test\"\n    motto: \"A test.\"\n    clade: \"deterministic/pure\"\n    proto_status: \"draft\"\n    aliases: [\"d1\"]\n  }}\n  lineage: {{\n    parents: [\"a\", \"b\"]\n    generated_by: \"dummy-test\"\n  }}\n  lang: \"rust\"\n}};"
        )
        .unwrap();

        let id = parse_holon(&path).unwrap();
        assert_eq!(id.uuid, "abc-123");
        assert_eq!(id.given_name, "test");
        assert_eq!(id.lang, "rust");
        assert_eq!(id.parents, vec!["a".to_string(), "b".to_string()]);
        assert_eq!(id.generated_by, "dummy-test");
        assert_eq!(id.proto_status, "draft");
        assert_eq!(id.aliases, vec!["d1".to_string()]);

        fs::remove_file(&path).unwrap();
    }

    #[test]
    fn test_resolve_proto_file_reads_manifest_fields() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("holon.proto");
        let mut file = fs::File::create(&path).unwrap();
        writeln!(
            file,
            "syntax = \"proto3\";\n\npackage test.v1;\n\noption (holons.v1.manifest) = {{\n  identity: {{\n    uuid: \"echo-123\"\n    given_name: \"Echo\"\n    family_name: \"Holon\"\n    motto: \"Replies clearly.\"\n    composer: \"identity-test\"\n    status: \"draft\"\n    born: \"2026-03-23\"\n    version: \"1.2.3\"\n    aliases: [\"echo-holon\"]\n  }}\n  description: \"Build-time manifest metadata.\"\n  lang: \"rust\"\n  skills: {{\n    name: \"echo\"\n    description: \"Echo a payload.\"\n    when: \"The caller needs a quick check.\"\n    steps: [\"Call Ping\"]\n  }}\n  kind: \"native\"\n  build: {{\n    runner: \"cargo\"\n    main: \"./cmd\"\n    members: {{ path: \"members/daemon\" }}\n  }}\n  requires: {{\n    files: [\"Cargo.toml\"]\n  }}\n  artifacts: {{\n    binary: \"echo-holon\"\n    primary: \"echo.holon\"\n  }}\n  sequences: {{\n    name: \"ping-once\"\n    description: \"Ping the holon once.\"\n    params: {{\n      name: \"message\"\n      description: \"Message to echo.\"\n      required: true\n      default: \"hello\"\n    }}\n    steps: [\"op echo-holon Ping\"]\n  }}\n}};"
        )
        .unwrap();

        let resolved = resolve_proto_file(&path).unwrap();
        assert_eq!(resolved.description, "Build-time manifest metadata.");
        assert_eq!(resolved.build_runner, "cargo");
        assert_eq!(resolved.build_main, "./cmd");
        assert_eq!(resolved.required_files, vec!["Cargo.toml".to_string()]);
        assert_eq!(resolved.member_paths, vec!["members/daemon".to_string()]);
        assert_eq!(resolved.skills.len(), 1);
        assert_eq!(resolved.skills[0].name, "echo");
        assert_eq!(resolved.sequences.len(), 1);
        assert_eq!(resolved.sequences[0].params.len(), 1);
        assert!(resolved.sequences[0].params[0].required);
        assert_eq!(resolved.sequences[0].params[0].default, "hello");
    }

    #[test]
    fn test_parse_invalid_mapping() {
        let dir = std::env::temp_dir();
        let path = dir.join("invalid_holon_rust.proto");
        fs::write(&path, "syntax = \"proto3\";\n\npackage test.v1;\n").unwrap();
        assert!(parse_holon(&path).is_err());
        fs::remove_file(&path).unwrap();
    }
}
