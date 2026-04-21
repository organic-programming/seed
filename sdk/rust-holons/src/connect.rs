use crate::command_channel::{
    connect_stdio_transport, endpoint_with_timeout, start_stdio_command, stop_child,
};
use crate::discover;
use crate::discovery_types::{ConnectResult, HolonInfo, HolonRef, LOCAL};
use reqwest::Url;
use std::collections::HashMap;
use std::env;
use std::path::{Path, PathBuf};
use std::sync::{Mutex, OnceLock};
use tokio::process::Child;
use tonic::transport::Channel;

struct ProcessOwner {
    child: Mutex<Option<Child>>,
}

struct LaunchTarget {
    command_path: PathBuf,
    args_prefix: Vec<String>,
    cwd: Option<PathBuf>,
}

pub async fn connect(
    scope: i32,
    expression: &str,
    root: Option<&str>,
    specifiers: i32,
    timeout: u32,
) -> ConnectResult {
    if scope != LOCAL {
        return ConnectResult {
            channel: None,
            uid: String::new(),
            origin: None,
            error: Some(format!("scope {scope} not supported")),
        };
    }

    let target = expression.trim();
    if target.is_empty() {
        return ConnectResult {
            channel: None,
            uid: String::new(),
            origin: None,
            error: Some("expression is required".to_string()),
        };
    }

    let resolved = discover::resolve(scope, target, root, specifiers, timeout);
    if let Some(error) = resolved.error {
        return ConnectResult {
            channel: None,
            uid: String::new(),
            origin: resolved.r#ref,
            error: Some(error),
        };
    }

    let Some(holon_ref) = resolved.r#ref else {
        return ConnectResult {
            channel: None,
            uid: String::new(),
            origin: None,
            error: Some(format!("holon \"{target}\" not found")),
        };
    };

    if let Some(error) = holon_ref.error.clone() {
        return ConnectResult {
            channel: None,
            uid: String::new(),
            origin: Some(holon_ref),
            error: Some(error),
        };
    }

    match connect_resolved(&holon_ref, timeout).await {
        Ok(channel) => ConnectResult {
            channel: Some(channel),
            uid: String::new(),
            origin: Some(holon_ref),
            error: None,
        },
        Err(error) => ConnectResult {
            channel: None,
            uid: String::new(),
            origin: Some(holon_ref),
            error: Some(error),
        },
    }
}

pub fn disconnect(mut result: ConnectResult) {
    if let Some(channel) = result.channel.take() {
        drop(channel);
    }
    let Some(origin) = result.origin.take() else {
        return;
    };
    if let Some(owner) = take_started(&origin.url) {
        let _ = owner.stop();
    }
}

async fn connect_resolved(holon_ref: &HolonRef, timeout: u32) -> Result<Channel, String> {
    match url_scheme(&holon_ref.url).as_str() {
        "tcp" => connect_tcp_url(&holon_ref.url, timeout),
        "http" | "https" => endpoint_with_timeout(&holon_ref.url, timeout)?
            .connect()
            .await
            .map_err(|err| err.to_string()),
        "file" => {
            let target = launch_target_from_ref(holon_ref)?;
            let mut args = target.args_prefix;
            args.extend([
                "serve".to_string(),
                "--listen".to_string(),
                "stdio://".to_string(),
            ]);

            let (transport, mut child) =
                start_stdio_command(&target.command_path, &args, target.cwd.as_deref())?;
            let channel = match connect_stdio_transport(transport, timeout).await {
                Ok(channel) => channel,
                Err(error) => {
                    let _ = stop_child(&mut child);
                    return Err(error);
                }
            };

            if let Err(error) = remember_started(holon_ref.url.clone(), child) {
                drop(channel);
                return Err(error);
            }
            Ok(channel)
        }
        _ => Err(format!("unsupported target URL {:?}", holon_ref.url)),
    }
}

fn launch_target_from_ref(holon_ref: &HolonRef) -> Result<LaunchTarget, String> {
    let path = path_from_file_url(&holon_ref.url)?;
    let info = holon_ref
        .info
        .as_ref()
        .ok_or_else(|| "holon metadata unavailable".to_string())?;

    if path.is_file() {
        return Ok(LaunchTarget {
            command_path: path.clone(),
            args_prefix: Vec::new(),
            cwd: path.parent().map(Path::to_path_buf),
        });
    }
    if !path.is_dir() {
        return Err(format!("target path {:?} is not launchable", path));
    }

    if path
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or_default()
        .ends_with(".holon")
    {
        if let Some(target) = package_launch_target(&path, info)? {
            return Ok(target);
        }
    }

    if let Some(target) = source_launch_target(&path, info)? {
        return Ok(target);
    }

    Err("target unreachable".to_string())
}

fn package_launch_target(
    package_dir: &Path,
    info: &HolonInfo,
) -> Result<Option<LaunchTarget>, String> {
    let entrypoint = effective_entrypoint(info);
    if entrypoint.is_empty() {
        return Ok(None);
    }

    let entry_name = Path::new(&entrypoint)
        .file_name()
        .ok_or_else(|| format!("invalid entrypoint {:?}", entrypoint))?;
    let binary_path = package_dir
        .join("bin")
        .join(package_arch_dir())
        .join(entry_name);
    if binary_path.is_file() {
        return Ok(Some(LaunchTarget {
            command_path: binary_path,
            args_prefix: Vec::new(),
            cwd: Some(package_dir.to_path_buf()),
        }));
    }

    let git_root = package_dir.join("git");
    if git_root.is_dir() {
        return source_launch_target(&git_root, info);
    }

    Ok(None)
}

fn source_launch_target(
    source_dir: &Path,
    info: &HolonInfo,
) -> Result<Option<LaunchTarget>, String> {
    let entrypoint = effective_entrypoint(info);
    if entrypoint.is_empty() {
        return Ok(None);
    }

    let entry_name = Path::new(&entrypoint)
        .file_name()
        .ok_or_else(|| format!("invalid entrypoint {:?}", entrypoint))?;
    let absolute_entry = PathBuf::from(&entrypoint);
    if absolute_entry.is_absolute() && absolute_entry.is_file() {
        return Ok(Some(LaunchTarget {
            command_path: absolute_entry,
            args_prefix: Vec::new(),
            cwd: Some(source_dir.to_path_buf()),
        }));
    }

    let source_package_binary = source_dir
        .join(".op")
        .join("build")
        .join(format!("{}.holon", info.slug))
        .join("bin")
        .join(package_arch_dir())
        .join(entry_name);
    if source_package_binary.is_file() {
        return Ok(Some(LaunchTarget {
            command_path: source_package_binary,
            args_prefix: Vec::new(),
            cwd: Some(source_dir.to_path_buf()),
        }));
    }

    let source_binary = source_dir
        .join(".op")
        .join("build")
        .join("bin")
        .join(entry_name);
    if source_binary.is_file() {
        return Ok(Some(LaunchTarget {
            command_path: source_binary,
            args_prefix: Vec::new(),
            cwd: Some(source_dir.to_path_buf()),
        }));
    }

    let direct_entry = source_dir.join(&entrypoint);
    if direct_entry.is_file() {
        if let Some(target) = runner_launch_target(info, &direct_entry, source_dir) {
            return Ok(Some(target));
        }
        return Ok(Some(LaunchTarget {
            command_path: direct_entry,
            args_prefix: Vec::new(),
            cwd: Some(source_dir.to_path_buf()),
        }));
    }

    Ok(None)
}

fn runner_launch_target(info: &HolonInfo, entrypoint: &Path, cwd: &Path) -> Option<LaunchTarget> {
    let runner = info.runner.trim().to_lowercase();
    let entrypoint = entrypoint.to_string_lossy().to_string();
    let cwd = Some(cwd.to_path_buf());

    match runner.as_str() {
        "go" | "go-module" => Some(LaunchTarget {
            command_path: PathBuf::from("go"),
            args_prefix: vec!["run".to_string(), entrypoint],
            cwd,
        }),
        "python" => Some(LaunchTarget {
            command_path: PathBuf::from(
                env::var("PYTHON").unwrap_or_else(|_| "python3".to_string()),
            ),
            args_prefix: vec![entrypoint],
            cwd,
        }),
        "node" | "typescript" | "npm" => Some(LaunchTarget {
            command_path: PathBuf::from("node"),
            args_prefix: vec![entrypoint],
            cwd,
        }),
        "ruby" => Some(LaunchTarget {
            command_path: PathBuf::from("ruby"),
            args_prefix: vec![entrypoint],
            cwd,
        }),
        "dart" => Some(LaunchTarget {
            command_path: PathBuf::from("dart"),
            args_prefix: vec!["run".to_string(), entrypoint],
            cwd,
        }),
        _ => None,
    }
}

fn effective_entrypoint(info: &HolonInfo) -> String {
    if info.entrypoint.trim().is_empty() {
        info.slug.clone()
    } else {
        info.entrypoint.trim().to_string()
    }
}

fn connect_tcp_url(raw_url: &str, timeout: u32) -> Result<Channel, String> {
    let parsed = crate::transport::parse_uri(raw_url).map_err(|err| err.to_string())?;
    let host = match parsed.host.as_deref() {
        Some("" | "0.0.0.0" | "::") | None => "127.0.0.1",
        Some(host) => host,
    };
    let port = parsed
        .port
        .ok_or_else(|| format!("invalid tcp target {:?}: missing port", raw_url))?;
    let endpoint = format!("http://{host}:{port}");

    run_connect_direct(endpoint, timeout)
}

fn run_connect_direct(endpoint: String, timeout: u32) -> Result<Channel, String> {
    if tokio::runtime::Handle::try_current().is_ok() {
        let (tx, rx) = std::sync::mpsc::sync_channel(1);
        std::thread::spawn(move || {
            let runtime = tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
                .unwrap();
            let result = runtime.block_on(async {
                endpoint_with_timeout(&endpoint, timeout)?
                    .connect()
                    .await
                    .map_err(|err| err.to_string())
            });
            let _ = tx.send(result);
        });
        return rx.recv().map_err(|err| err.to_string())?;
    }

    tokio::runtime::Builder::new_current_thread()
        .enable_all()
        .build()
        .map_err(|err| err.to_string())?
        .block_on(async {
            endpoint_with_timeout(&endpoint, timeout)?
                .connect()
                .await
                .map_err(|err| err.to_string())
        })
}

fn url_scheme(raw_url: &str) -> String {
    Url::parse(raw_url.trim())
        .map(|url| url.scheme().to_string())
        .unwrap_or_default()
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

fn started_registry() -> &'static Mutex<HashMap<String, Vec<ProcessOwner>>> {
    static REGISTRY: OnceLock<Mutex<HashMap<String, Vec<ProcessOwner>>>> = OnceLock::new();
    REGISTRY.get_or_init(|| Mutex::new(HashMap::new()))
}

fn remember_started(key: String, child: Child) -> Result<(), String> {
    let owner = ProcessOwner {
        child: Mutex::new(Some(child)),
    };
    started_registry()
        .lock()
        .map_err(|_| "started registry poisoned".to_string())?
        .entry(key)
        .or_default()
        .push(owner);
    Ok(())
}

fn take_started(key: &str) -> Option<ProcessOwner> {
    let mut guard = started_registry().lock().ok()?;
    let owners = guard.get_mut(key)?;
    let owner = owners.pop();
    if owners.is_empty() {
        guard.remove(key);
    }
    owner
}

impl ProcessOwner {
    fn stop(self) -> Result<(), String> {
        if let Some(mut child) = self.child.lock().unwrap().take() {
            return stop_child(&mut child);
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::gen::holons::v1::{holon_meta_client::HolonMetaClient, DescribeRequest};
    use crate::test_support::{
        acquire_process_guard, file_url, write_package_holon, PackageSeed, ProcessStateGuard,
    };
    use tempfile::TempDir;

    #[tokio::test]
    async fn test_connect_unresolvable() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();

        let result = connect(
            LOCAL,
            "missing",
            Some(runtime.root_str()),
            crate::INSTALLED,
            1000,
        )
        .await;
        assert!(result.error.is_some());
        assert!(result.channel.is_none());
        assert!(result.origin.is_none());
    }

    #[tokio::test]
    async fn test_connect_returns_connect_result() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.op_bin.join("known-slug.holon"),
            &package_seed("known-slug", "uuid-known", "Known", "Slug"),
            true,
            true,
        );

        let result = connect(
            LOCAL,
            "known-slug",
            Some(runtime.root_str()),
            crate::INSTALLED,
            5000,
        )
        .await;
        assert!(result.error.is_none());
        assert!(result.channel.is_some());
        assert_eq!(result.uid, "");

        let mut client = HolonMetaClient::new(result.channel.as_ref().unwrap().clone());
        let response = client
            .describe(DescribeRequest {})
            .await
            .unwrap()
            .into_inner();
        assert_eq!(
            response
                .manifest
                .as_ref()
                .and_then(|manifest| manifest.identity.as_ref())
                .map(|identity| identity.given_name.clone()),
            Some("Fixture".to_string())
        );

        disconnect(result);
    }

    #[tokio::test]
    async fn test_connect_returns_origin() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        let package_root = runtime.op_bin.join("origin-slug.holon");
        write_package_holon(
            &package_root,
            &package_seed("origin-slug", "uuid-origin", "Origin", "Slug"),
            true,
            true,
        );

        let result = connect(
            LOCAL,
            "origin-slug",
            Some(runtime.root_str()),
            crate::INSTALLED,
            5000,
        )
        .await;
        assert!(result.error.is_none());
        assert_eq!(
            result
                .origin
                .as_ref()
                .and_then(|holon_ref| holon_ref.info.as_ref())
                .map(|info| info.slug.clone()),
            Some("origin-slug".to_string())
        );
        assert_eq!(
            result
                .origin
                .as_ref()
                .map(|holon_ref| holon_ref.url.clone()),
            Some(file_url(&package_root))
        );

        disconnect(result);
    }

    #[tokio::test]
    async fn test_disconnect_accepts_connect_result() {
        let _lock = acquire_process_guard().await;
        let _state = ProcessStateGuard::capture();
        let runtime = runtime_fixture();
        write_package_holon(
            &runtime.op_bin.join("disconnect-slug.holon"),
            &package_seed("disconnect-slug", "uuid-disconnect", "Disconnect", "Slug"),
            true,
            true,
        );

        let result = connect(
            LOCAL,
            "disconnect-slug",
            Some(runtime.root_str()),
            crate::INSTALLED,
            5000,
        )
        .await;
        assert!(result.error.is_none());
        assert!(result.channel.is_some());
        disconnect(result);
    }

    struct RuntimeFixture {
        tempdir: TempDir,
        op_bin: PathBuf,
    }

    impl RuntimeFixture {
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
        RuntimeFixture { tempdir, op_bin }
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
}
