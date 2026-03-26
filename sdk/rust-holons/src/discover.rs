//! Discover holons by scanning for holon.proto manifests.

use crate::identity::{self, HolonIdentity};
use std::collections::{HashMap, HashSet};
use std::env;
use std::error::Error;
use std::fs;
use std::path::{Path, PathBuf};

pub type Result<T> = std::result::Result<T, Box<dyn Error>>;

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Manifest {
    pub kind: String,
    pub build: Build,
    pub artifacts: Artifacts,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Build {
    pub runner: String,
    pub main: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Artifacts {
    pub binary: String,
    pub primary: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HolonEntry {
    pub slug: String,
    pub uuid: String,
    pub dir: PathBuf,
    pub relative_path: String,
    pub origin: String,
    pub identity: HolonIdentity,
    pub manifest: Option<Manifest>,
}

pub fn discover(root: &Path) -> Result<Vec<HolonEntry>> {
    discover_in_root(root, "local")
}

pub fn discover_local() -> Result<Vec<HolonEntry>> {
    discover(&current_root()?)
}

pub fn discover_all() -> Result<Vec<HolonEntry>> {
    let roots = vec![
        (current_root()?, "local"),
        (opbin(), "$OPBIN"),
        (cache_dir(), "cache"),
    ];

    let mut seen = HashSet::new();
    let mut entries = Vec::new();
    for (root, origin) in roots {
        for entry in discover_in_root(&root, origin)? {
            let key = if entry.uuid.trim().is_empty() {
                entry.dir.display().to_string()
            } else {
                entry.uuid.clone()
            };
            if seen.insert(key) {
                entries.push(entry);
            }
        }
    }
    Ok(entries)
}

pub fn find_by_slug(slug: &str) -> Result<Option<HolonEntry>> {
    let needle = slug.trim();
    if needle.is_empty() {
        return Ok(None);
    }

    let mut matched: Option<HolonEntry> = None;
    for entry in discover_all()? {
        if entry.slug != needle {
            continue;
        }
        if let Some(existing) = &matched {
            if existing.uuid != entry.uuid {
                return Err(format!("ambiguous holon \"{needle}\"").into());
            }
        } else {
            matched = Some(entry);
        }
    }

    Ok(matched)
}

pub fn find_by_uuid(prefix: &str) -> Result<Option<HolonEntry>> {
    let needle = prefix.trim();
    if needle.is_empty() {
        return Ok(None);
    }

    let mut matched: Option<HolonEntry> = None;
    for entry in discover_all()? {
        if !entry.uuid.starts_with(needle) {
            continue;
        }
        if let Some(existing) = &matched {
            if existing.uuid != entry.uuid {
                return Err(format!("ambiguous UUID prefix \"{needle}\"").into());
            }
        } else {
            matched = Some(entry);
        }
    }

    Ok(matched)
}

fn discover_in_root(root: &Path, origin: &str) -> Result<Vec<HolonEntry>> {
    let trimmed = if root.as_os_str().is_empty() {
        current_root()?
    } else {
        root.to_path_buf()
    };
    let abs_root = match trimmed.canonicalize() {
        Ok(path) => path,
        Err(_) => trimmed.clone(),
    };
    let metadata = match fs::metadata(&abs_root) {
        Ok(metadata) => metadata,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(Vec::new()),
        Err(error) => return Err(error.into()),
    };
    if !metadata.is_dir() {
        return Ok(Vec::new());
    }

    let mut entries_by_key = HashMap::new();
    let mut ordered_keys = Vec::new();
    scan_dir(
        &abs_root,
        &abs_root,
        origin,
        &mut entries_by_key,
        &mut ordered_keys,
    )?;

    let mut entries = Vec::new();
    for key in ordered_keys {
        if let Some(entry) = entries_by_key.remove(&key) {
            entries.push(entry);
        }
    }

    entries.sort_by(|left, right| {
        left.relative_path
            .cmp(&right.relative_path)
            .then(left.uuid.cmp(&right.uuid))
    });
    Ok(entries)
}

fn scan_dir(
    root: &Path,
    dir: &Path,
    origin: &str,
    entries_by_key: &mut HashMap<String, HolonEntry>,
    ordered_keys: &mut Vec<String>,
) -> Result<()> {
    for child in fs::read_dir(dir)? {
        let child = match child {
            Ok(child) => child,
            Err(_) => continue,
        };
        let path = child.path();
        let file_type = match child.file_type() {
            Ok(file_type) => file_type,
            Err(_) => continue,
        };
        let name = child.file_name();
        let name = name.to_string_lossy();

        if file_type.is_dir() {
            if should_skip_dir(root, &path, &name) {
                continue;
            }
            scan_dir(root, &path, origin, entries_by_key, ordered_keys)?;
            continue;
        }

        if !file_type.is_file() || name != identity::PROTO_MANIFEST_FILE_NAME {
            continue;
        }

        let resolved = match identity::resolve_proto_file(&path) {
            Ok(resolved) => resolved,
            Err(_) => continue,
        };
        let dir = manifest_root(&path);
        let abs_dir = match dir.canonicalize() {
            Ok(path) => path,
            Err(_) => dir.clone(),
        };
        let entry = HolonEntry {
            slug: resolved.identity.slug(),
            uuid: resolved.identity.uuid.clone(),
            dir: abs_dir.clone(),
            relative_path: relative_path(root, &abs_dir),
            origin: origin.to_string(),
            identity: resolved.identity.clone(),
            manifest: Some(Manifest {
                kind: resolved.kind.clone(),
                build: Build {
                    runner: resolved.build_runner.clone(),
                    main: resolved.build_main.clone(),
                },
                artifacts: Artifacts {
                    binary: resolved.artifact_binary.clone(),
                    primary: resolved.artifact_primary.clone(),
                },
            }),
        };

        let key = if entry.uuid.trim().is_empty() {
            entry.dir.display().to_string()
        } else {
            entry.uuid.clone()
        };
        if let Some(existing) = entries_by_key.get(&key) {
            if path_depth(&entry.relative_path) < path_depth(&existing.relative_path) {
                entries_by_key.insert(key, entry);
            }
            continue;
        }

        ordered_keys.push(key.clone());
        entries_by_key.insert(key, entry);
    }

    Ok(())
}

fn should_skip_dir(root: &Path, path: &Path, name: &str) -> bool {
    if path == root {
        return false;
    }
    matches!(
        name,
        ".git" | ".op" | "node_modules" | "vendor" | "build" | "testdata"
    ) || name.starts_with('.')
}

fn relative_path(root: &Path, dir: &Path) -> String {
    match dir.strip_prefix(root) {
        Ok(relative) if relative.as_os_str().is_empty() => ".".to_string(),
        Ok(relative) => relative.to_string_lossy().replace('\\', "/"),
        Err(_) => dir.to_string_lossy().replace('\\', "/"),
    }
}

fn manifest_root(path: &Path) -> PathBuf {
    let manifest_dir = path
        .parent()
        .map(Path::to_path_buf)
        .unwrap_or_else(|| PathBuf::from("."));
    let version_dir = manifest_dir
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default();
    let api_dir = manifest_dir
        .parent()
        .and_then(|parent| parent.file_name())
        .and_then(|name| name.to_str())
        .unwrap_or_default();
    if regex::Regex::new(r"^v[0-9]+(?:[A-Za-z0-9._-]*)?$")
        .unwrap()
        .is_match(version_dir)
        && api_dir == "api"
    {
        return manifest_dir
            .parent()
            .and_then(|parent| parent.parent())
            .map(Path::to_path_buf)
            .unwrap_or(manifest_dir);
    }
    manifest_dir
}

fn path_depth(relative_path: &str) -> usize {
    let trimmed = relative_path.trim().trim_matches('/');
    if trimmed.is_empty() || trimmed == "." {
        return 0;
    }
    trimmed.split('/').count()
}

fn current_root() -> Result<PathBuf> {
    Ok(env::current_dir()?)
}

fn oppath() -> PathBuf {
    if let Ok(path) = env::var("OPPATH") {
        let trimmed = path.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }

    if let Ok(home) = env::var("HOME") {
        let trimmed = home.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed).join(".op");
        }
    }

    PathBuf::from(".op")
}

fn opbin() -> PathBuf {
    if let Ok(path) = env::var("OPBIN") {
        let trimmed = path.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    oppath().join("bin")
}

fn cache_dir() -> PathBuf {
    oppath().join("cache")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::test_support::{acquire_process_guard_blocking, ProcessStateGuard};
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn test_discover_recurses_skips_and_dedups() {
        let root = temp_dir("discover-rust");
        write_holon(
            &root.join("holons/alpha"),
            HolonSeed::new("uuid-alpha", "Alpha", "Go", "alpha-go"),
        );
        write_holon(
            &root.join("nested/beta"),
            HolonSeed::new("uuid-beta", "Beta", "Rust", "beta-rust"),
        );
        write_holon(
            &root.join("nested/dup/alpha"),
            HolonSeed::new("uuid-alpha", "Alpha", "Go", "alpha-go"),
        );

        for skipped in [
            root.join(".git/hidden"),
            root.join(".op/hidden"),
            root.join("node_modules/hidden"),
            root.join("vendor/hidden"),
            root.join("build/hidden"),
            root.join(".cache/hidden"),
        ] {
            write_holon(
                &skipped,
                HolonSeed::new("ignored-uuid", "Ignored", "Holon", "ignored-holon"),
            );
        }

        let entries = discover(&root).unwrap();
        assert_eq!(entries.len(), 2);

        let alpha = entries
            .iter()
            .find(|entry| entry.uuid == "uuid-alpha")
            .unwrap();
        assert_eq!(alpha.slug, "alpha-go");
        assert_eq!(alpha.relative_path, "holons/alpha");
        assert_eq!(
            alpha.manifest.as_ref().unwrap().build.runner,
            "go-module".to_string()
        );

        let beta = entries
            .iter()
            .find(|entry| entry.uuid == "uuid-beta")
            .unwrap();
        assert_eq!(beta.relative_path, "nested/beta");
    }

    #[test]
    fn test_discover_all_prefers_local_root() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();

        let root = temp_dir("discover-all-rust");
        let local = root.join("local");
        let op_home = root.join("runtime");
        let op_bin = op_home.join("bin");
        let cache = op_home.join("cache");

        write_holon(
            &local.join("rob-go"),
            HolonSeed::new("same-uuid", "Rob", "Go", "rob-go"),
        );
        write_holon(
            &op_bin.join("rob-go"),
            HolonSeed::new("same-uuid", "Rob", "Go", "rob-go"),
        );
        write_holon(
            &cache.join("deps/rob-go"),
            HolonSeed::new("same-uuid", "Rob", "Go", "rob-go"),
        );

        env::set_var("OPPATH", op_home.as_os_str());
        env::set_var("OPBIN", op_bin.as_os_str());
        env::set_current_dir(&local).unwrap();

        let entries = discover_all().unwrap();
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].origin, "local");
    }

    #[test]
    fn test_find_by_slug_and_uuid() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();

        let root = temp_dir("find-rust");
        let op_home = root.join("runtime");
        let op_bin = op_home.join("bin");

        write_holon(
            &root.join("rob-go"),
            HolonSeed::new(
                "c7f3a1b2-1111-1111-1111-111111111111",
                "Rob",
                "Go",
                "rob-go",
            ),
        );

        env::set_var("OPPATH", op_home.as_os_str());
        env::set_var("OPBIN", op_bin.as_os_str());
        env::set_current_dir(&root).unwrap();

        let by_slug = find_by_slug("rob-go").unwrap().unwrap();
        assert_eq!(by_slug.uuid, "c7f3a1b2-1111-1111-1111-111111111111");

        let by_uuid = find_by_uuid("c7f3a1b2").unwrap().unwrap();
        assert_eq!(by_uuid.slug, "rob-go");

        assert!(find_by_slug("missing").unwrap().is_none());

        let _ = fs::remove_dir_all(root);
    }

    fn temp_dir(prefix: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let dir = env::temp_dir().join(format!("{prefix}-{unique}"));
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    fn write_holon(dir: &Path, seed: HolonSeed) {
        fs::create_dir_all(dir).unwrap();
        fs::write(
            dir.join("holon.proto"),
            format!(
                "syntax = \"proto3\";\n\npackage test.v1;\n\noption (holons.v1.manifest) = {{\n  identity: {{\n    uuid: \"{}\"\n    given_name: \"{}\"\n    family_name: \"{}\"\n    motto: \"Test\"\n    composer: \"test\"\n    clade: \"deterministic/pure\"\n    status: \"draft\"\n    born: \"2026-03-07\"\n  }}\n  lineage: {{\n    generated_by: \"test\"\n  }}\n  kind: \"native\"\n  build: {{\n    runner: \"go-module\"\n  }}\n  artifacts: {{\n    binary: \"{}\"\n  }}\n}};\n",
                seed.uuid, seed.given_name, seed.family_name, seed.binary
            ),
        )
        .unwrap();
    }

    struct HolonSeed {
        uuid: String,
        given_name: String,
        family_name: String,
        binary: String,
    }

    impl HolonSeed {
        fn new(uuid: &str, given_name: &str, family_name: &str, binary: &str) -> Self {
            Self {
                uuid: uuid.to_string(),
                given_name: given_name.to_string(),
                family_name: family_name.to_string(),
                binary: binary.to_string(),
            }
        }
    }
}
