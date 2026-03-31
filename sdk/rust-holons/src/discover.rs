use crate::command_channel::{connect_stdio_transport, start_stdio_command, stop_child};
use crate::discovery_types::{
    DiscoverResult, HolonInfo, HolonRef, IdentityInfo, ResolveResult, ALL, BUILT, CACHED, CWD,
    INSTALLED, LOCAL, NO_LIMIT, SIBLINGS, SOURCE,
};
use crate::gen::holons::v1::{
    holon_meta_client::HolonMetaClient, DescribeRequest, DescribeResponse,
};
use reqwest::Url;
use serde::Deserialize;
use std::collections::{HashMap, HashSet};
use std::env;
use std::fs;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::process::{Command as ProcessCommand, Output, Stdio};
#[cfg(test)]
use std::sync::{Mutex, OnceLock};

const EXCLUDED_DIRS: &[&str] = &[".git", ".op", "node_modules", "vendor", "build", "testdata"];

#[derive(Debug, Clone)]
pub(crate) struct DiscoveredEntry {
    holon_ref: HolonRef,
    dir_path: PathBuf,
    relative_path: String,
}

#[derive(Debug, Deserialize, Default)]
struct PackageJSON {
    #[serde(default)]
    schema: String,
    #[serde(default)]
    slug: String,
    #[serde(default)]
    uuid: String,
    #[serde(default)]
    identity: PackageIdentityJSON,
    #[serde(default)]
    lang: String,
    #[serde(default)]
    runner: String,
    #[serde(default)]
    status: String,
    #[serde(default)]
    kind: String,
    #[serde(default)]
    transport: String,
    #[serde(default)]
    entrypoint: String,
    #[serde(default)]
    architectures: Vec<String>,
    #[serde(default)]
    has_dist: bool,
    #[serde(default)]
    has_source: bool,
}

#[derive(Debug, Deserialize, Default)]
struct PackageIdentityJSON {
    #[serde(default)]
    given_name: String,
    #[serde(default)]
    family_name: String,
    #[serde(default)]
    motto: String,
    #[serde(default)]
    aliases: Vec<String>,
}

#[derive(Debug, Deserialize, Default)]
struct OPDiscoverOutput {
    #[serde(default)]
    entries: Vec<OPDiscoverEntry>,
}

#[derive(Debug, Deserialize, Default)]
struct OPDiscoverEntry {
    #[serde(default)]
    slug: String,
    #[serde(default)]
    uuid: String,
    #[serde(default, alias = "givenName")]
    given_name: String,
    #[serde(default, alias = "familyName")]
    family_name: String,
    #[serde(default)]
    lang: String,
    #[serde(default)]
    status: String,
    #[serde(default, alias = "relativePath")]
    relative_path: String,
    #[serde(default)]
    origin: String,
}

type SourceDiscoverBridgeFn = fn(
    scope: i32,
    expression: Option<&str>,
    root: &Path,
    specifiers: i32,
    limit: i32,
    timeout: u32,
) -> DiscoverResult;
type PackageProbeFn =
    fn(root: &Path, dir: &Path, origin: &str, timeout: u32) -> Result<DiscoveredEntry, String>;

pub fn discover(
    scope: i32,
    expression: Option<&str>,
    root: Option<&str>,
    specifiers: i32,
    limit: i32,
    timeout: u32,
) -> DiscoverResult {
    if scope != LOCAL {
        return DiscoverResult {
            found: Vec::new(),
            error: Some(format!("scope {scope} not supported")),
        };
    }
    if specifiers < 0 || specifiers & !ALL != 0 {
        return DiscoverResult {
            found: Vec::new(),
            error: Some(format!(
                "invalid specifiers 0x{specifiers:02X}: valid range is 0x00-0x3F"
            )),
        };
    }

    let specifiers = if specifiers == 0 { ALL } else { specifiers };
    if limit < 0 {
        return DiscoverResult {
            found: Vec::new(),
            error: None,
        };
    }

    let expression = normalized_expression(expression);
    let mut search_root: Option<PathBuf> = None;
    let mut resolve_root = || -> Result<PathBuf, String> {
        if let Some(root) = &search_root {
            return Ok(root.clone());
        }
        let resolved = resolve_discover_root(root)?;
        search_root = Some(resolved.clone());
        Ok(resolved)
    };

    if let Some(expression) = expression.as_deref() {
        match discover_path_expression(expression, &mut resolve_root, timeout) {
            Ok((refs, true)) => {
                return DiscoverResult {
                    found: apply_ref_limit(refs, limit),
                    error: None,
                }
            }
            Ok((_, false)) => {}
            Err(error) => {
                return DiscoverResult {
                    found: Vec::new(),
                    error: Some(error),
                }
            }
        }
    }

    let search_root = match resolve_root() {
        Ok(root) => root,
        Err(error) => {
            return DiscoverResult {
                found: Vec::new(),
                error: Some(error),
            }
        }
    };

    let entries = match discover_entries(&search_root, specifiers, timeout) {
        Ok(entries) => entries,
        Err(error) => {
            return DiscoverResult {
                found: Vec::new(),
                error: Some(error),
            }
        }
    };

    let mut found = Vec::new();
    for entry in entries {
        if !matches_expression(&entry, expression.as_deref()) {
            continue;
        }
        found.push(entry.holon_ref);
        if limit > 0 && found.len() >= limit as usize {
            break;
        }
    }

    DiscoverResult { found, error: None }
}

pub fn resolve(
    scope: i32,
    expression: &str,
    root: Option<&str>,
    specifiers: i32,
    timeout: u32,
) -> ResolveResult {
    let result = discover(scope, Some(expression), root, specifiers, 1, timeout);
    if let Some(error) = result.error {
        return ResolveResult {
            r#ref: None,
            error: Some(error),
        };
    }
    let Some(holon_ref) = result.found.into_iter().next() else {
        return ResolveResult {
            r#ref: None,
            error: Some(format!("holon \"{expression}\" not found")),
        };
    };

    if let Some(error) = holon_ref.error.clone() {
        return ResolveResult {
            r#ref: Some(holon_ref),
            error: Some(error),
        };
    }

    ResolveResult {
        r#ref: Some(holon_ref),
        error: None,
    }
}

fn discover_entries(
    root: &Path,
    specifiers: i32,
    timeout: u32,
) -> Result<Vec<DiscoveredEntry>, String> {
    let mut seen = HashSet::new();
    let mut found = Vec::new();

    for (flag, name) in [
        (SIBLINGS, "siblings"),
        (CWD, "cwd"),
        (SOURCE, "source"),
        (BUILT, "built"),
        (INSTALLED, "installed"),
        (CACHED, "cached"),
    ] {
        if specifiers & flag == 0 {
            continue;
        }

        let entries = match flag {
            SIBLINGS => match bundle_holons_root() {
                Some(bundle_root) => discover_packages_direct(&bundle_root, name, timeout)?,
                None => Vec::new(),
            },
            CWD => discover_packages_recursive(root, name, timeout)?,
            SOURCE => discover_source_entries(root, timeout)?,
            BUILT => discover_packages_direct(&root.join(".op").join("build"), name, timeout)?,
            INSTALLED => discover_packages_direct(&opbin(), name, timeout)?,
            CACHED => discover_packages_recursive(&cache_dir(), name, timeout)?,
            _ => Vec::new(),
        };

        for entry in entries {
            let key = entry_key(&entry);
            if seen.insert(key) {
                found.push(entry);
            }
        }
    }

    Ok(found)
}

fn discover_packages_direct(
    root: &Path,
    origin: &str,
    timeout: u32,
) -> Result<Vec<DiscoveredEntry>, String> {
    discover_packages_from_dirs(root, origin, package_dirs_direct(root)?, timeout)
}

fn discover_packages_recursive(
    root: &Path,
    origin: &str,
    timeout: u32,
) -> Result<Vec<DiscoveredEntry>, String> {
    discover_packages_from_dirs(root, origin, package_dirs_recursive(root)?, timeout)
}

fn discover_packages_from_dirs(
    root: &Path,
    origin: &str,
    dirs: Vec<PathBuf>,
    timeout: u32,
) -> Result<Vec<DiscoveredEntry>, String> {
    let abs_root = normalize_search_root(root)?;
    let mut entries_by_key = HashMap::new();
    let mut ordered_keys = Vec::new();

    for dir in dirs {
        let entry = match load_package_entry(&abs_root, &dir, origin) {
            Ok(entry) => entry,
            Err(_) => match probe_package_entry(&abs_root, &dir, origin, timeout) {
                Ok(entry) => entry,
                Err(_) => continue,
            },
        };

        let key = entry_key(&entry);
        if let Some(existing) = entries_by_key.get(&key) {
            if should_replace_entry(existing, &entry) {
                entries_by_key.insert(key, entry);
            }
            continue;
        }

        entries_by_key.insert(key.clone(), entry);
        ordered_keys.push(key);
    }

    let mut entries = ordered_keys
        .into_iter()
        .filter_map(|key| entries_by_key.remove(&key))
        .collect::<Vec<_>>();
    entries.sort_by(|left, right| {
        left.relative_path
            .cmp(&right.relative_path)
            .then(entry_sort_key(left).cmp(&entry_sort_key(right)))
    });
    Ok(entries)
}

fn load_package_entry(root: &Path, dir: &Path, _origin: &str) -> Result<DiscoveredEntry, String> {
    let data = fs::read(dir.join(".holon.json")).map_err(|err| err.to_string())?;
    let payload: PackageJSON = serde_json::from_slice(&data).map_err(|err| err.to_string())?;
    let schema = payload.schema.trim();
    if !schema.is_empty() && schema != "holon-package/v1" {
        return Err(format!("unsupported package schema {schema:?}"));
    }

    let abs_dir = absolute_path(dir)?;
    let identity = IdentityInfo {
        given_name: payload.identity.given_name.trim().to_string(),
        family_name: payload.identity.family_name.trim().to_string(),
        motto: payload.identity.motto.trim().to_string(),
        aliases: compact_strings(payload.identity.aliases),
    };
    let slug = if payload.slug.trim().is_empty() {
        slug_for(&identity.given_name, &identity.family_name)
    } else {
        payload.slug.trim().to_string()
    };
    let info = HolonInfo {
        slug,
        uuid: payload.uuid.trim().to_string(),
        identity,
        lang: payload.lang.trim().to_string(),
        runner: payload.runner.trim().to_string(),
        status: payload.status.trim().to_string(),
        kind: payload.kind.trim().to_string(),
        transport: payload.transport.trim().to_string(),
        entrypoint: payload.entrypoint.trim().to_string(),
        architectures: compact_strings(payload.architectures),
        has_dist: payload.has_dist,
        has_source: payload.has_source,
    };

    Ok(DiscoveredEntry {
        holon_ref: HolonRef {
            url: file_url(&abs_dir),
            info: Some(info),
            error: None,
        },
        dir_path: abs_dir.clone(),
        relative_path: relative_path(root, &abs_dir),
    })
}

fn probe_package_entry(
    root: &Path,
    dir: &Path,
    origin: &str,
    timeout: u32,
) -> Result<DiscoveredEntry, String> {
    package_probe_bridge()(root, dir, origin, timeout)
}

fn native_probe_package_entry(
    root: &Path,
    dir: &Path,
    _origin: &str,
    timeout: u32,
) -> Result<DiscoveredEntry, String> {
    let mut info = describe_package_directory(dir, timeout)?;
    let abs_dir = absolute_path(dir)?;
    info.has_dist = abs_dir.join("dist").is_dir() || info.has_dist;
    info.has_source = abs_dir.join("git").is_dir() || info.has_source;
    if info.architectures.is_empty() {
        info.architectures = package_architectures(&abs_dir)?;
    }

    Ok(DiscoveredEntry {
        holon_ref: HolonRef {
            url: file_url(&abs_dir),
            info: Some(info),
            error: None,
        },
        dir_path: abs_dir.clone(),
        relative_path: relative_path(root, &abs_dir),
    })
}

fn describe_package_directory(dir: &Path, timeout: u32) -> Result<HolonInfo, String> {
    describe_binary_target(&package_binary_path(dir)?, timeout)
}

fn describe_binary_target(binary_path: &Path, timeout: u32) -> Result<HolonInfo, String> {
    run_future_blocking(describe_binary_target_async(
        binary_path.to_path_buf(),
        timeout,
    ))
}

async fn describe_binary_target_async(
    binary_path: PathBuf,
    timeout: u32,
) -> Result<HolonInfo, String> {
    let args = vec![
        "serve".to_string(),
        "--listen".to_string(),
        "stdio://".to_string(),
    ];
    let (transport, mut child) = start_stdio_command(&binary_path, &args, None)?;
    let channel = match connect_stdio_transport(transport, timeout).await {
        Ok(channel) => channel,
        Err(error) => {
            let _ = stop_child(&mut child);
            return Err(error);
        }
    };

    let response = describe_channel(channel.clone(), timeout).await;
    drop(channel);
    let stop_result = stop_child(&mut child);

    response.and_then(|response| {
        stop_result?;
        holon_info_from_describe_response(response)
    })
}

async fn describe_channel(
    channel: tonic::transport::Channel,
    timeout: u32,
) -> Result<DescribeResponse, String> {
    let mut client = HolonMetaClient::new(channel);
    if timeout == 0 {
        return client
            .describe(DescribeRequest {})
            .await
            .map(|response| response.into_inner())
            .map_err(|err| err.to_string());
    }

    tokio::time::timeout(
        std::time::Duration::from_millis(timeout as u64),
        client.describe(DescribeRequest {}),
    )
    .await
    .map_err(|_| "discover timed out".to_string())?
    .map(|response| response.into_inner())
    .map_err(|err| err.to_string())
}

fn holon_info_from_describe_response(response: DescribeResponse) -> Result<HolonInfo, String> {
    let manifest = response
        .manifest
        .ok_or_else(|| "Describe returned no manifest".to_string())?;
    let identity = manifest
        .identity
        .ok_or_else(|| "Describe returned no manifest identity".to_string())?;
    let slug = slug_for(&identity.given_name, &identity.family_name);

    Ok(HolonInfo {
        slug,
        uuid: identity.uuid,
        identity: IdentityInfo {
            given_name: identity.given_name,
            family_name: identity.family_name,
            motto: identity.motto,
            aliases: identity.aliases,
        },
        lang: manifest.lang,
        runner: manifest.build.map(|build| build.runner).unwrap_or_default(),
        status: identity.status,
        kind: manifest.kind,
        transport: manifest.transport,
        entrypoint: manifest
            .artifacts
            .map(|artifacts| artifacts.binary)
            .unwrap_or_default(),
        architectures: manifest.platforms,
        has_dist: false,
        has_source: false,
    })
}

fn discover_source_entries(root: &Path, timeout: u32) -> Result<Vec<DiscoveredEntry>, String> {
    let result = source_discover_bridge()(LOCAL, None, root, SOURCE, NO_LIMIT, timeout);
    if let Some(error) = result.error {
        return Err(error);
    }
    Ok(entries_from_refs(root, result.found))
}

fn discover_path_expression<F>(
    expression: &str,
    resolve_root: &mut F,
    timeout: u32,
) -> Result<(Vec<HolonRef>, bool), String>
where
    F: FnMut() -> Result<PathBuf, String>,
{
    let Some(candidate) = path_expression_candidate(expression, resolve_root)? else {
        return Ok((Vec::new(), false));
    };
    let Some(holon_ref) = discover_ref_at_path(&candidate, timeout)? else {
        return Ok((Vec::new(), true));
    };
    Ok((vec![holon_ref], true))
}

fn path_expression_candidate<F>(
    expression: &str,
    resolve_root: &mut F,
) -> Result<Option<PathBuf>, String>
where
    F: FnMut() -> Result<PathBuf, String>,
{
    let trimmed = expression.trim();
    if trimmed.is_empty() {
        return Ok(None);
    }

    if trimmed.to_lowercase().starts_with("file://") {
        return path_from_file_url(trimmed).map(Some);
    }
    if trimmed.contains("://") {
        return Ok(None);
    }
    if !(Path::new(trimmed).is_absolute()
        || trimmed.starts_with('.')
        || trimmed.contains(std::path::MAIN_SEPARATOR)
        || trimmed.contains('/')
        || trimmed.contains('\\')
        || trimmed.to_lowercase().ends_with(".holon"))
    {
        return Ok(None);
    }

    if Path::new(trimmed).is_absolute() {
        return Ok(Some(PathBuf::from(trimmed)));
    }
    Ok(Some(resolve_root()?.join(trimmed)))
}

fn discover_ref_at_path(path: &Path, timeout: u32) -> Result<Option<HolonRef>, String> {
    let abs_path = absolute_path(path)?;
    if !abs_path.exists() {
        return Ok(None);
    }

    if abs_path.is_dir() {
        if abs_path
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or_default()
            .ends_with(".holon")
            || abs_path.join(".holon.json").is_file()
        {
            let root = abs_path
                .parent()
                .map(Path::to_path_buf)
                .unwrap_or_else(|| abs_path.clone());
            return match load_package_entry(&root, &abs_path, "path") {
                Ok(entry) => Ok(Some(entry.holon_ref)),
                Err(_) => match probe_package_entry(&root, &abs_path, "path", timeout) {
                    Ok(entry) => Ok(Some(entry.holon_ref)),
                    Err(error) => Ok(Some(HolonRef {
                        url: file_url(&abs_path),
                        info: None,
                        error: Some(error),
                    })),
                },
            };
        }

        let result = source_discover_bridge()(LOCAL, None, &abs_path, SOURCE, NO_LIMIT, timeout);
        if let Some(error) = result.error {
            return Err(error);
        }
        if result.found.len() == 1 {
            return Ok(result.found.into_iter().next());
        }
        for holon_ref in &result.found {
            if path_from_ref_url(&holon_ref.url).as_deref() == Some(abs_path.as_path()) {
                return Ok(Some(holon_ref.clone()));
            }
        }
        return Ok(None);
    }

    if abs_path.file_name().and_then(|name| name.to_str()) == Some("holon.proto") {
        let parent = abs_path
            .parent()
            .map(Path::to_path_buf)
            .unwrap_or_else(|| abs_path.clone());
        let result = source_discover_bridge()(LOCAL, None, &parent, SOURCE, NO_LIMIT, timeout);
        if let Some(error) = result.error {
            return Err(error);
        }
        if result.found.len() == 1 {
            return Ok(result.found.into_iter().next());
        }
        for holon_ref in &result.found {
            if path_from_ref_url(&holon_ref.url).as_deref() == Some(parent.as_path()) {
                return Ok(Some(holon_ref.clone()));
            }
        }
        return Ok(None);
    }

    match describe_binary_target(&abs_path, timeout) {
        Ok(info) => Ok(Some(HolonRef {
            url: file_url(&abs_path),
            info: Some(info),
            error: None,
        })),
        Err(error) => Ok(Some(HolonRef {
            url: file_url(&abs_path),
            info: None,
            error: Some(error),
        })),
    }
}

fn entries_from_refs(root: &Path, refs: Vec<HolonRef>) -> Vec<DiscoveredEntry> {
    refs.into_iter()
        .map(|holon_ref| {
            let dir_path =
                path_from_ref_url(&holon_ref.url).unwrap_or_else(|| PathBuf::from(&holon_ref.url));
            DiscoveredEntry {
                relative_path: relative_path(root, &dir_path),
                dir_path,
                holon_ref,
            }
        })
        .collect()
}

fn matches_expression(entry: &DiscoveredEntry, expression: Option<&str>) -> bool {
    let Some(expression) = expression else {
        return true;
    };
    let needle = expression.trim();
    if needle.is_empty() {
        return false;
    }

    if let Some(info) = &entry.holon_ref.info {
        if info.slug == needle || info.uuid.starts_with(needle) {
            return true;
        }
        if info.identity.aliases.iter().any(|alias| alias == needle) {
            return true;
        }
    }

    let mut base = entry
        .dir_path
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default()
        .to_string();
    if base.ends_with(".holon") {
        base.truncate(base.len() - ".holon".len());
    }
    base == needle
}

fn normalized_expression(expression: Option<&str>) -> Option<String> {
    expression.map(|value| value.trim().to_string())
}

fn entry_key(entry: &DiscoveredEntry) -> String {
    entry
        .holon_ref
        .info
        .as_ref()
        .map(|info| info.uuid.trim())
        .filter(|uuid| !uuid.is_empty())
        .map(|uuid| uuid.to_string())
        .unwrap_or_else(|| entry.dir_path.to_string_lossy().to_string())
}

fn entry_sort_key(entry: &DiscoveredEntry) -> String {
    entry
        .holon_ref
        .info
        .as_ref()
        .map(|info| info.uuid.trim())
        .filter(|uuid| !uuid.is_empty())
        .map(|uuid| uuid.to_string())
        .unwrap_or_else(|| entry.holon_ref.url.clone())
}

fn should_replace_entry(current: &DiscoveredEntry, next_entry: &DiscoveredEntry) -> bool {
    path_depth(&next_entry.relative_path) < path_depth(&current.relative_path)
}

fn package_dirs_direct(root: &Path) -> Result<Vec<PathBuf>, String> {
    let abs_root = normalize_search_root(root)?;
    if !abs_root.is_dir() {
        return Ok(Vec::new());
    }

    let mut dirs = fs::read_dir(&abs_root)
        .map_err(|err| err.to_string())?
        .filter_map(Result::ok)
        .filter_map(|entry| {
            let path = entry.path();
            path.is_dir().then_some(path).filter(|path| {
                path.file_name()
                    .and_then(|name| name.to_str())
                    .unwrap_or_default()
                    .ends_with(".holon")
            })
        })
        .collect::<Vec<_>>();
    dirs.sort();
    Ok(dirs)
}

fn package_dirs_recursive(root: &Path) -> Result<Vec<PathBuf>, String> {
    let abs_root = normalize_search_root(root)?;
    if !abs_root.is_dir() {
        return Ok(Vec::new());
    }

    let mut dirs = Vec::new();
    walk_package_dirs(&abs_root, &abs_root, &mut dirs)?;
    dirs.sort();
    Ok(dirs)
}

fn walk_package_dirs(root: &Path, current: &Path, out: &mut Vec<PathBuf>) -> Result<(), String> {
    let mut children = fs::read_dir(current)
        .map_err(|err| err.to_string())?
        .filter_map(Result::ok)
        .collect::<Vec<_>>();
    children.sort_by_key(|entry| entry.path());

    for child in children {
        let path = child.path();
        let file_type = match child.file_type() {
            Ok(file_type) => file_type,
            Err(_) => continue,
        };
        if !file_type.is_dir() {
            continue;
        }

        let name = child.file_name();
        let name = name.to_string_lossy();
        if name.ends_with(".holon") {
            out.push(path);
            continue;
        }
        if should_skip_dir(root, &path, &name) {
            continue;
        }
        walk_package_dirs(root, &path, out)?;
    }

    Ok(())
}

fn should_skip_dir(root: &Path, path: &Path, name: &str) -> bool {
    if path == root {
        return false;
    }
    if name.ends_with(".holon") {
        return false;
    }
    EXCLUDED_DIRS.contains(&name) || name.starts_with('.')
}

fn resolve_discover_root(root: Option<&str>) -> Result<PathBuf, String> {
    match root {
        None => env::current_dir().map_err(|err| err.to_string()),
        Some(value) => {
            let trimmed = value.trim();
            if trimmed.is_empty() {
                return Err("root cannot be empty".to_string());
            }
            let root = absolute_path(Path::new(trimmed))?;
            if !root.is_dir() {
                return Err(format!("root \"{trimmed}\" is not a directory"));
            }
            Ok(root)
        }
    }
}

fn normalize_search_root(root: &Path) -> Result<PathBuf, String> {
    if root.as_os_str().is_empty() {
        return env::current_dir().map_err(|err| err.to_string());
    }
    absolute_path(root)
}

fn relative_path(root: &Path, path: &Path) -> String {
    match path.strip_prefix(root) {
        Ok(relative) if relative.as_os_str().is_empty() => ".".to_string(),
        Ok(relative) => relative.to_string_lossy().replace('\\', "/"),
        Err(_) => path.to_string_lossy().replace('\\', "/"),
    }
}

fn path_depth(relative_path: &str) -> usize {
    let trimmed = relative_path.trim().trim_matches('/');
    if trimmed.is_empty() || trimmed == "." {
        return 0;
    }
    trimmed.split('/').count()
}

fn bundle_holons_root() -> Option<PathBuf> {
    let mut current = current_executable()?.parent()?.to_path_buf();
    loop {
        if current
            .file_name()
            .and_then(|name| name.to_str())
            .unwrap_or_default()
            .ends_with(".app")
        {
            let candidate = current.join("Contents").join("Resources").join("Holons");
            if candidate.is_dir() {
                return Some(candidate);
            }
        }
        let parent = current.parent()?.to_path_buf();
        if parent == current {
            return None;
        }
        current = parent;
    }
}

fn current_executable() -> Option<PathBuf> {
    #[cfg(test)]
    if let Some(path) = executable_override_cell().lock().unwrap().clone() {
        return Some(path);
    }
    env::current_exe().ok()
}

fn oppath() -> PathBuf {
    env::var("OPPATH")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .map(PathBuf::from)
        .or_else(|| {
            env::var("HOME")
                .ok()
                .map(|home| PathBuf::from(home).join(".op"))
        })
        .unwrap_or_else(|| PathBuf::from(".op"))
}

fn opbin() -> PathBuf {
    env::var("OPBIN")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| oppath().join("bin"))
}

fn cache_dir() -> PathBuf {
    oppath().join("cache")
}

fn package_binary_path(dir: &Path) -> Result<PathBuf, String> {
    let arch_dir = dir.join("bin").join(package_arch_dir());
    if !arch_dir.is_dir() {
        return Err(format!("no package binary for arch {}", package_arch_dir()));
    }
    let mut candidates = fs::read_dir(&arch_dir)
        .map_err(|err| err.to_string())?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.is_file())
        .collect::<Vec<_>>();
    candidates.sort();
    candidates
        .into_iter()
        .next()
        .ok_or_else(|| format!("no package binary for arch {}", package_arch_dir()))
}

fn package_architectures(dir: &Path) -> Result<Vec<String>, String> {
    let bin_root = dir.join("bin");
    if !bin_root.is_dir() {
        return Ok(Vec::new());
    }
    let mut architectures = fs::read_dir(bin_root)
        .map_err(|err| err.to_string())?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| path.is_dir())
        .filter_map(|path| {
            path.file_name()
                .and_then(|name| name.to_str())
                .map(str::to_string)
        })
        .collect::<Vec<_>>();
    architectures.sort();
    Ok(architectures)
}

fn package_arch_dir() -> String {
    let os = match env::consts::OS {
        "macos" => "darwin",
        other => other,
    };
    let arch = match env::consts::ARCH {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        other => other,
    };
    format!("{os}_{arch}")
}

fn apply_ref_limit(refs: Vec<HolonRef>, limit: i32) -> Vec<HolonRef> {
    if limit <= 0 || refs.len() <= limit as usize {
        return refs;
    }
    refs.into_iter().take(limit as usize).collect()
}

fn slug_for(given_name: &str, family_name: &str) -> String {
    let given = given_name.trim();
    let family = family_name.trim().trim_end_matches('?');
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

fn compact_strings(values: Vec<String>) -> Vec<String> {
    values
        .into_iter()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .collect()
}

fn file_url(path: &Path) -> String {
    if let Ok(url) = Url::from_file_path(path) {
        return url.to_string();
    }
    format!("file://{}", path.to_string_lossy().replace('\\', "/"))
}

fn path_from_ref_url(raw_url: &str) -> Option<PathBuf> {
    raw_url
        .to_lowercase()
        .starts_with("file://")
        .then(|| path_from_file_url(raw_url).ok())
        .flatten()
}

fn path_from_file_url(raw_url: &str) -> Result<PathBuf, String> {
    let parsed = Url::parse(raw_url.trim()).map_err(|err| err.to_string())?;
    if parsed.scheme() != "file" {
        return Err(format!("holon URL {raw_url:?} is not a local file target"));
    }
    parsed
        .to_file_path()
        .map_err(|_| format!("holon URL {raw_url:?} has no path"))
}

fn absolute_path(path: &Path) -> Result<PathBuf, String> {
    let absolute = if path.is_absolute() {
        path.to_path_buf()
    } else {
        env::current_dir()
            .map_err(|err| err.to_string())?
            .join(path)
    };

    fs::canonicalize(&absolute)
        .or_else(|_| Ok(absolute))
        .map_err(|err: std::io::Error| err.to_string())
}

fn run_future_blocking<F, T>(future: F) -> Result<T, String>
where
    F: std::future::Future<Output = Result<T, String>> + Send + 'static,
    T: Send + 'static,
{
    if tokio::runtime::Handle::try_current().is_ok() {
        let (tx, rx) = std::sync::mpsc::sync_channel(1);
        std::thread::spawn(move || {
            let runtime = tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
                .unwrap();
            let _ = tx.send(runtime.block_on(future));
        });
        return rx.recv().map_err(|err| err.to_string())?;
    }

    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|err| err.to_string())?
        .block_on(future)
}

fn source_discover_bridge() -> SourceDiscoverBridgeFn {
    #[cfg(test)]
    {
        return *source_discover_bridge_cell().lock().unwrap();
    }

    #[cfg(not(test))]
    {
        default_source_discover_bridge
    }
}

fn package_probe_bridge() -> PackageProbeFn {
    #[cfg(test)]
    {
        return *package_probe_bridge_cell().lock().unwrap();
    }

    #[cfg(not(test))]
    {
        native_probe_package_entry
    }
}

fn default_source_discover_bridge(
    scope: i32,
    expression: Option<&str>,
    root: &Path,
    specifiers: i32,
    limit: i32,
    timeout: u32,
) -> DiscoverResult {
    if scope != LOCAL {
        return DiscoverResult {
            found: Vec::new(),
            error: Some(format!("scope {scope} not supported")),
        };
    }
    if specifiers != SOURCE {
        return DiscoverResult {
            found: Vec::new(),
            error: Some(format!(
                "invalid source bridge specifiers 0x{specifiers:02X}"
            )),
        };
    }
    if limit < 0 {
        return DiscoverResult {
            found: Vec::new(),
            error: None,
        };
    }

    let Some(op_binary) = find_on_path("op") else {
        return DiscoverResult {
            found: Vec::new(),
            error: None,
        };
    };

    let output = match run_op_discover(&op_binary, root, timeout) {
        Ok(Some(output)) => output,
        Ok(None) | Err(_) => {
            return DiscoverResult {
                found: Vec::new(),
                error: None,
            }
        }
    };
    if !output.status.success() {
        return DiscoverResult {
            found: Vec::new(),
            error: None,
        };
    }

    let payload = match serde_json::from_slice::<OPDiscoverOutput>(&output.stdout) {
        Ok(payload) => payload,
        Err(_) => {
            return DiscoverResult {
                found: Vec::new(),
                error: None,
            }
        }
    };

    let mut refs = payload
        .entries
        .into_iter()
        .filter(|entry| entry.origin.trim() == "source")
        .map(|entry| {
            let identity = IdentityInfo {
                given_name: entry.given_name.trim().to_string(),
                family_name: entry.family_name.trim().to_string(),
                motto: String::new(),
                aliases: Vec::new(),
            };
            let path = if entry.relative_path.trim().is_empty() {
                root.to_path_buf()
            } else {
                root.join(entry.relative_path.trim())
            };
            HolonRef {
                url: file_url(&path),
                info: Some(HolonInfo {
                    slug: if entry.slug.trim().is_empty() {
                        slug_for(&identity.given_name, &identity.family_name)
                    } else {
                        entry.slug.trim().to_string()
                    },
                    uuid: entry.uuid.trim().to_string(),
                    identity,
                    lang: entry.lang.trim().to_string(),
                    runner: String::new(),
                    status: entry.status.trim().to_string(),
                    kind: String::new(),
                    transport: String::new(),
                    entrypoint: String::new(),
                    architectures: Vec::new(),
                    has_dist: false,
                    has_source: true,
                }),
                error: None,
            }
        })
        .collect::<Vec<_>>();

    if let Some(expression) = expression {
        refs = entries_from_refs(root, refs)
            .into_iter()
            .filter(|entry| matches_expression(entry, Some(expression)))
            .map(|entry| entry.holon_ref)
            .collect();
    }

    DiscoverResult {
        found: apply_ref_limit(refs, limit),
        error: None,
    }
}

fn run_op_discover(op_binary: &Path, root: &Path, timeout: u32) -> Result<Option<Output>, String> {
    let mut command = ProcessCommand::new(op_binary);
    command
        .arg("discover")
        .arg("--json")
        .current_dir(root)
        .env("OPROOT", root)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());

    if timeout == 0 {
        return command.output().map(Some).map_err(|err| err.to_string());
    }

    let mut child = command.spawn().map_err(|err| err.to_string())?;
    let deadline = std::time::Instant::now() + std::time::Duration::from_millis(timeout as u64);
    loop {
        match child.try_wait().map_err(|err| err.to_string())? {
            Some(status) => return collect_output(&mut child, status).map(Some),
            None if std::time::Instant::now() >= deadline => {
                let _ = child.kill();
                let _ = child.wait();
                return Ok(None);
            }
            None => std::thread::sleep(std::time::Duration::from_millis(25)),
        }
    }
}

fn collect_output(
    child: &mut std::process::Child,
    status: std::process::ExitStatus,
) -> Result<Output, String> {
    let mut stdout = Vec::new();
    let mut stderr = Vec::new();

    if let Some(mut pipe) = child.stdout.take() {
        pipe.read_to_end(&mut stdout)
            .map_err(|err| err.to_string())?;
    }
    if let Some(mut pipe) = child.stderr.take() {
        pipe.read_to_end(&mut stderr)
            .map_err(|err| err.to_string())?;
    }

    Ok(Output {
        status,
        stdout,
        stderr,
    })
}

fn find_on_path(binary_name: &str) -> Option<PathBuf> {
    let path_var = env::var_os("PATH")?;
    env::split_paths(&path_var)
        .map(|dir| dir.join(binary_name))
        .find(|candidate| candidate.is_file())
}

#[cfg(test)]
fn source_discover_bridge_cell() -> &'static Mutex<SourceDiscoverBridgeFn> {
    static CELL: OnceLock<Mutex<SourceDiscoverBridgeFn>> = OnceLock::new();
    CELL.get_or_init(|| Mutex::new(default_source_discover_bridge))
}

#[cfg(test)]
fn package_probe_bridge_cell() -> &'static Mutex<PackageProbeFn> {
    static CELL: OnceLock<Mutex<PackageProbeFn>> = OnceLock::new();
    CELL.get_or_init(|| Mutex::new(native_probe_package_entry))
}

#[cfg(test)]
fn executable_override_cell() -> &'static Mutex<Option<PathBuf>> {
    static CELL: OnceLock<Mutex<Option<PathBuf>>> = OnceLock::new();
    CELL.get_or_init(|| Mutex::new(None))
}

#[cfg(test)]
pub(crate) fn set_source_discover_bridge(bridge: SourceDiscoverBridgeFn) {
    *source_discover_bridge_cell().lock().unwrap() = bridge;
}

#[cfg(test)]
pub(crate) fn reset_source_discover_bridge() {
    *source_discover_bridge_cell().lock().unwrap() = default_source_discover_bridge;
}

#[cfg(test)]
pub(crate) fn set_package_probe_bridge(bridge: PackageProbeFn) {
    *package_probe_bridge_cell().lock().unwrap() = bridge;
}

#[cfg(test)]
pub(crate) fn reset_package_probe_bridge() {
    *package_probe_bridge_cell().lock().unwrap() = native_probe_package_entry;
}

#[cfg(test)]
pub(crate) fn set_current_executable_override(path: Option<PathBuf>) {
    *executable_override_cell().lock().unwrap() = path;
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::test_support::{
        acquire_process_guard_blocking, compiled_fixture_holon_binary,
        file_url as support_file_url, write_package_holon, PackageSeed, ProcessStateGuard,
    };
    use std::sync::atomic::{AtomicUsize, Ordering};
    use tempfile::TempDir;

    static PROBE_CALLS: AtomicUsize = AtomicUsize::new(0);

    #[test]
    fn test_discover_all_layers() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        reset_source_discover_bridge();
        reset_package_probe_bridge();
        set_current_executable_override(None);

        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("cwd-alpha.holon"),
            &package_seed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"),
            true,
            false,
        );
        write_package_holon(
            &runtime
                .root()
                .join(".op")
                .join("build")
                .join("built-beta.holon"),
            &package_seed("built-beta", "uuid-built-beta", "Built", "Beta"),
            true,
            false,
        );
        write_package_holon(
            &runtime.op_bin.join("installed-gamma.holon"),
            &package_seed(
                "installed-gamma",
                "uuid-installed-gamma",
                "Installed",
                "Gamma",
            ),
            true,
            false,
        );
        write_package_holon(
            &runtime
                .op_home
                .join("cache")
                .join("deps")
                .join("cached-delta.holon"),
            &package_seed("cached-delta", "uuid-cached-delta", "Cached", "Delta"),
            true,
            false,
        );

        let app_executable = runtime
            .root()
            .join("TestApp.app")
            .join("Contents")
            .join("MacOS")
            .join("TestApp");
        fs::create_dir_all(app_executable.parent().unwrap()).unwrap();
        fs::write(&app_executable, "#!/bin/sh\n").unwrap();
        write_package_holon(
            &runtime
                .root()
                .join("TestApp.app")
                .join("Contents")
                .join("Resources")
                .join("Holons")
                .join("bundle.holon"),
            &package_seed("bundle", "uuid-bundle", "Bundle", "Holon"),
            true,
            false,
        );
        set_current_executable_override(Some(app_executable));

        let result = discover(LOCAL, None, Some(runtime.root_str()), ALL, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(
            sorted_slugs(&result),
            vec![
                "built-beta",
                "bundle",
                "cached-delta",
                "cwd-alpha",
                "installed-gamma"
            ]
        );
    }

    #[test]
    fn test_discover_filter_by_specifiers() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();

        write_package_holon(
            &runtime.root().join("cwd-alpha.holon"),
            &package_seed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"),
            true,
            false,
        );
        write_package_holon(
            &runtime
                .root()
                .join(".op")
                .join("build")
                .join("built-beta.holon"),
            &package_seed("built-beta", "uuid-built-beta", "Built", "Beta"),
            true,
            false,
        );
        write_package_holon(
            &runtime.op_bin.join("installed-gamma.holon"),
            &package_seed(
                "installed-gamma",
                "uuid-installed-gamma",
                "Installed",
                "Gamma",
            ),
            true,
            false,
        );

        let result = discover(
            LOCAL,
            None,
            Some(runtime.root_str()),
            BUILT | INSTALLED,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["built-beta", "installed-gamma"]);
    }

    #[test]
    fn test_discover_match_by_slug() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );
        write_package_holon(
            &runtime.root().join("beta.holon"),
            &package_seed("beta", "uuid-beta", "Beta", "Two"),
            true,
            false,
        );

        let result = discover(
            LOCAL,
            Some("beta"),
            Some(runtime.root_str()),
            CWD,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["beta"]);
    }

    #[test]
    fn test_discover_match_by_alias() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let mut seed = package_seed("alpha", "uuid-alpha", "Alpha", "One");
        seed.aliases = vec!["first".to_string()];
        write_package_holon(&runtime.root().join("alpha.holon"), &seed, true, false);

        let result = discover(
            LOCAL,
            Some("first"),
            Some(runtime.root_str()),
            CWD,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["alpha"]);
    }

    #[test]
    fn test_discover_match_by_uuid_prefix() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "12345678-aaaa", "Alpha", "One"),
            true,
            false,
        );

        let result = discover(
            LOCAL,
            Some("12345678"),
            Some(runtime.root_str()),
            CWD,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["alpha"]);
    }

    #[test]
    fn test_discover_match_by_path() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let package_dir = runtime.root().join("nested").join("alpha.holon");
        write_package_holon(
            &package_dir,
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );

        let result = discover(
            LOCAL,
            Some("nested/alpha.holon"),
            Some(runtime.root_str()),
            CWD,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert_eq!(result.found.len(), 1);
        assert_eq!(result.found[0].url, support_file_url(&package_dir));
    }

    #[test]
    fn test_discover_limit_one() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );
        write_package_holon(
            &runtime.root().join("beta.holon"),
            &package_seed("beta", "uuid-beta", "Beta", "Two"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, 1, 0);
        assert_eq!(result.error, None);
        assert_eq!(result.found.len(), 1);
    }

    #[test]
    fn test_discover_limit_zero_means_unlimited() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );
        write_package_holon(
            &runtime.root().join("beta.holon"),
            &package_seed("beta", "uuid-beta", "Beta", "Two"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, 0, 0);
        assert_eq!(result.error, None);
        assert_eq!(result.found.len(), 2);
    }

    #[test]
    fn test_discover_negative_limit_returns_empty() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, -1, 0);
        assert_eq!(result.error, None);
        assert!(result.found.is_empty());
    }

    #[test]
    fn test_discover_invalid_specifiers() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let result = discover(LOCAL, None, Some(runtime.root_str()), 0xFF, NO_LIMIT, 0);
        assert!(result.error.is_some());
        assert!(result.found.is_empty());
    }

    #[test]
    fn test_discover_specifiers_zero_treated_as_all() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("cwd-alpha.holon"),
            &package_seed("cwd-alpha", "uuid-cwd-alpha", "Cwd", "Alpha"),
            true,
            false,
        );
        write_package_holon(
            &runtime
                .root()
                .join(".op")
                .join("build")
                .join("built-beta.holon"),
            &package_seed("built-beta", "uuid-built-beta", "Built", "Beta"),
            true,
            false,
        );
        write_package_holon(
            &runtime.op_bin.join("installed-gamma.holon"),
            &package_seed(
                "installed-gamma",
                "uuid-installed-gamma",
                "Installed",
                "Gamma",
            ),
            true,
            false,
        );
        write_package_holon(
            &runtime.op_home.join("cache").join("cached-delta.holon"),
            &package_seed("cached-delta", "uuid-cached-delta", "Cached", "Delta"),
            true,
            false,
        );

        let all_result = discover(LOCAL, None, Some(runtime.root_str()), ALL, NO_LIMIT, 0);
        let zero_result = discover(LOCAL, None, Some(runtime.root_str()), 0, NO_LIMIT, 0);
        assert_eq!(all_result.error, None);
        assert_eq!(zero_result.error, None);
        assert_eq!(sorted_slugs(&all_result), sorted_slugs(&zero_result));
    }

    #[test]
    fn test_discover_null_expression_returns_all() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );
        write_package_holon(
            &runtime.root().join("beta.holon"),
            &package_seed("beta", "uuid-beta", "Beta", "Two"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(result.found.len(), 2);
    }

    #[test]
    fn test_discover_missing_expression_returns_empty() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );

        let result = discover(
            LOCAL,
            Some("missing"),
            Some(runtime.root_str()),
            CWD,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert!(result.found.is_empty());
    }

    #[test]
    fn test_discover_skips_excluded_dirs() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("kept").join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );

        for skipped in [
            runtime
                .root()
                .join(".git")
                .join("hidden")
                .join("ignored.holon"),
            runtime
                .root()
                .join(".op")
                .join("hidden")
                .join("ignored.holon"),
            runtime
                .root()
                .join("node_modules")
                .join("hidden")
                .join("ignored.holon"),
            runtime
                .root()
                .join("vendor")
                .join("hidden")
                .join("ignored.holon"),
            runtime
                .root()
                .join("build")
                .join("hidden")
                .join("ignored.holon"),
            runtime
                .root()
                .join("testdata")
                .join("hidden")
                .join("ignored.holon"),
            runtime
                .root()
                .join(".cache")
                .join("hidden")
                .join("ignored.holon"),
        ] {
            write_package_holon(
                &skipped,
                &package_seed(
                    &format!(
                        "ignored-{}",
                        skipped
                            .parent()
                            .unwrap()
                            .file_name()
                            .unwrap()
                            .to_string_lossy()
                    ),
                    &format!(
                        "uuid-{}",
                        skipped
                            .parent()
                            .unwrap()
                            .file_name()
                            .unwrap()
                            .to_string_lossy()
                    ),
                    "Ignored",
                    "Holon",
                ),
                true,
                false,
            );
        }

        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["alpha"]);
    }

    #[test]
    fn test_discover_deduplicates_by_uuid() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let cwd_path = runtime.root().join("alpha.holon");
        let built_path = runtime
            .root()
            .join(".op")
            .join("build")
            .join("alpha-built.holon");
        write_package_holon(
            &cwd_path,
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );
        write_package_holon(
            &built_path,
            &package_seed("alpha-built", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), ALL, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(result.found.len(), 1);
        assert_eq!(result.found[0].url, support_file_url(&cwd_path));
    }

    #[test]
    fn test_discover_holon_json_fast_path() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        PROBE_CALLS.store(0, Ordering::SeqCst);
        set_package_probe_bridge(counting_probe_bridge);
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, NO_LIMIT, 0);
        reset_package_probe_bridge();

        assert_eq!(result.error, None);
        assert_eq!(PROBE_CALLS.load(Ordering::SeqCst), 0);
    }

    #[test]
    fn test_discover_describe_fallback_when_holon_json_missing() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        reset_package_probe_bridge();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            false,
            true,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), CWD, NO_LIMIT, 5000);
        assert_eq!(result.error, None);
        assert_eq!(result.found.len(), 1);
        assert_eq!(
            result.found[0].info.as_ref().map(|info| info.slug.clone()),
            Some("fixture-holon".to_string())
        );
    }

    #[test]
    fn test_discover_siblings_layer() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();

        let app_executable = runtime
            .root()
            .join("TestApp.app")
            .join("Contents")
            .join("MacOS")
            .join("TestApp");
        fs::create_dir_all(app_executable.parent().unwrap()).unwrap();
        fs::write(&app_executable, "#!/bin/sh\n").unwrap();
        write_package_holon(
            &runtime
                .root()
                .join("TestApp.app")
                .join("Contents")
                .join("Resources")
                .join("Holons")
                .join("bundle.holon"),
            &package_seed("bundle", "uuid-bundle", "Bundle", "Holon"),
            true,
            false,
        );
        set_current_executable_override(Some(app_executable));

        let result = discover(LOCAL, None, Some(runtime.root_str()), SIBLINGS, NO_LIMIT, 0);
        set_current_executable_override(None);

        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["bundle"]);
    }

    #[test]
    fn test_discover_source_layer_offloads_to_local_op() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let source_dir = runtime.root().join("source-holon");
        fs::create_dir_all(&source_dir).unwrap();

        set_source_discover_bridge(fake_source_bridge);
        let result = discover(
            LOCAL,
            None,
            Some(runtime.root_str()),
            SOURCE,
            NO_LIMIT,
            5000,
        );
        reset_source_discover_bridge();

        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["source-alpha"]);

        let calls = SOURCE_BRIDGE_CALLS.get().unwrap().lock().unwrap().clone();
        assert_eq!(
            calls,
            vec![(
                LOCAL,
                None,
                fs::canonicalize(runtime.root()).unwrap_or_else(|_| runtime.root().to_path_buf()),
                SOURCE,
                NO_LIMIT,
                5000
            )]
        );
    }

    #[test]
    fn test_discover_built_layer() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join(".op").join("build").join("built.holon"),
            &package_seed("built", "uuid-built", "Built", "Holon"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), BUILT, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["built"]);
    }

    #[test]
    fn test_discover_installed_layer() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.op_bin.join("installed.holon"),
            &package_seed("installed", "uuid-installed", "Installed", "Holon"),
            true,
            false,
        );

        let result = discover(
            LOCAL,
            None,
            Some(runtime.root_str()),
            INSTALLED,
            NO_LIMIT,
            0,
        );
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["installed"]);
    }

    #[test]
    fn test_discover_cached_layer() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime
                .op_home
                .join("cache")
                .join("deep")
                .join("cached.holon"),
            &package_seed("cached", "uuid-cached", "Cached", "Holon"),
            true,
            false,
        );

        let result = discover(LOCAL, None, Some(runtime.root_str()), CACHED, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["cached"]);
    }

    #[test]
    fn test_discover_nil_root_defaults_to_cwd() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );
        env::set_current_dir(runtime.root()).unwrap();

        let result = discover(LOCAL, None, None, CWD, NO_LIMIT, 0);
        assert_eq!(result.error, None);
        assert_eq!(sorted_slugs(&result), vec!["alpha"]);
    }

    #[test]
    fn test_discover_empty_root_returns_error() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let result = discover(LOCAL, None, Some(""), ALL, NO_LIMIT, 0);
        assert!(result.error.is_some());
        assert!(result.found.is_empty());
    }

    #[test]
    fn test_discover_unsupported_scope_returns_error() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let proxy = discover(
            crate::PROXY,
            None,
            Some(runtime.root_str()),
            ALL,
            NO_LIMIT,
            0,
        );
        let delegated = discover(
            crate::DELEGATED,
            None,
            Some(runtime.root_str()),
            ALL,
            NO_LIMIT,
            0,
        );
        assert!(proxy.error.is_some());
        assert!(delegated.error.is_some());
    }

    #[test]
    fn test_resolve_known_slug() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.root().join("alpha.holon"),
            &package_seed("alpha", "uuid-alpha", "Alpha", "One"),
            true,
            false,
        );

        let result = resolve(LOCAL, "alpha", Some(runtime.root_str()), CWD, 0);
        assert_eq!(result.error, None);
        assert_eq!(
            result
                .r#ref
                .and_then(|holon_ref| holon_ref.info)
                .map(|info| info.slug),
            Some("alpha".to_string())
        );
    }

    #[test]
    fn test_resolve_missing() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let result = resolve(LOCAL, "missing", Some(runtime.root_str()), ALL, 0);
        assert!(result.error.is_some());
    }

    #[test]
    fn test_resolve_invalid_specifiers() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let result = resolve(LOCAL, "alpha", Some(runtime.root_str()), 0xFF, 0);
        assert!(result.error.is_some());
    }

    #[derive(Debug)]
    struct RuntimeFixture {
        tempdir: TempDir,
        op_home: PathBuf,
        op_bin: PathBuf,
    }

    impl RuntimeFixture {
        fn root(&self) -> &Path {
            self.tempdir.path()
        }

        fn root_str(&self) -> &str {
            self.tempdir.path().to_str().unwrap()
        }
    }

    fn runtime_fixture() -> RuntimeFixture {
        let tempdir = tempfile::tempdir().unwrap();
        let op_home = tempdir.path().join("runtime");
        let op_bin = op_home.join("bin");
        env::set_var("OPPATH", &op_home);
        env::set_var("OPBIN", &op_bin);
        RuntimeFixture {
            tempdir,
            op_home,
            op_bin,
        }
    }

    fn package_seed(slug: &str, uuid: &str, given_name: &str, family_name: &str) -> PackageSeed {
        PackageSeed {
            slug: slug.to_string(),
            uuid: uuid.to_string(),
            given_name: given_name.to_string(),
            family_name: family_name.to_string(),
            ..PackageSeed::default()
        }
    }

    fn sorted_slugs(result: &DiscoverResult) -> Vec<String> {
        let mut slugs = result
            .found
            .iter()
            .filter_map(|holon_ref| holon_ref.info.as_ref().map(|info| info.slug.clone()))
            .collect::<Vec<_>>();
        slugs.sort();
        slugs
    }

    fn counting_probe_bridge(
        root: &Path,
        dir: &Path,
        origin: &str,
        timeout: u32,
    ) -> Result<DiscoveredEntry, String> {
        PROBE_CALLS.fetch_add(1, Ordering::SeqCst);
        native_probe_package_entry(root, dir, origin, timeout)
    }

    static SOURCE_BRIDGE_CALLS: OnceLock<
        Mutex<Vec<(i32, Option<String>, PathBuf, i32, i32, u32)>>,
    > = OnceLock::new();

    fn fake_source_bridge(
        scope: i32,
        expression: Option<&str>,
        root: &Path,
        specifiers: i32,
        limit: i32,
        timeout: u32,
    ) -> DiscoverResult {
        SOURCE_BRIDGE_CALLS
            .get_or_init(|| Mutex::new(Vec::new()))
            .lock()
            .unwrap()
            .push((
                scope,
                expression.map(str::to_string),
                root.to_path_buf(),
                specifiers,
                limit,
                timeout,
            ));

        let source_dir = root.join("source-holon");
        DiscoverResult {
            found: vec![HolonRef {
                url: support_file_url(&source_dir),
                info: Some(HolonInfo {
                    slug: "source-alpha".to_string(),
                    uuid: "uuid-source-alpha".to_string(),
                    identity: IdentityInfo {
                        given_name: "Source".to_string(),
                        family_name: "Alpha".to_string(),
                        motto: String::new(),
                        aliases: Vec::new(),
                    },
                    lang: "rust".to_string(),
                    runner: "rust".to_string(),
                    status: "draft".to_string(),
                    kind: "native".to_string(),
                    transport: "stdio".to_string(),
                    entrypoint: "source-alpha".to_string(),
                    architectures: Vec::new(),
                    has_dist: false,
                    has_source: true,
                }),
                error: None,
            }],
            error: None,
        }
    }

    #[test]
    fn test_fixture_binary_exists() {
        let _lock = acquire_process_guard_blocking();
        let _state = ProcessStateGuard::capture();
        assert!(compiled_fixture_holon_binary().is_file());
    }
}
