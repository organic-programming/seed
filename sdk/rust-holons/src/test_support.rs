use std::env;
use std::fs;
use std::path::Path;
use std::path::PathBuf;
use std::process::Command;
use std::sync::OnceLock;
use tokio::sync::{Mutex, MutexGuard};

pub(crate) fn global_process_guard() -> &'static Mutex<()> {
    static GUARD: OnceLock<Mutex<()>> = OnceLock::new();
    GUARD.get_or_init(|| Mutex::new(()))
}

pub(crate) async fn acquire_process_guard() -> MutexGuard<'static, ()> {
    global_process_guard().lock().await
}

pub(crate) fn acquire_process_guard_blocking() -> MutexGuard<'static, ()> {
    global_process_guard().blocking_lock()
}

pub(crate) struct ProcessStateGuard {
    cwd: PathBuf,
    oppath: Option<String>,
    opbin: Option<String>,
}

impl ProcessStateGuard {
    pub(crate) fn capture() -> Self {
        Self {
            cwd: env::current_dir().unwrap(),
            oppath: env::var("OPPATH").ok(),
            opbin: env::var("OPBIN").ok(),
        }
    }
}

impl Drop for ProcessStateGuard {
    fn drop(&mut self) {
        let _ = env::set_current_dir(&self.cwd);
        match &self.oppath {
            Some(value) => env::set_var("OPPATH", value),
            None => env::remove_var("OPPATH"),
        }
        match &self.opbin {
            Some(value) => env::set_var("OPBIN", value),
            None => env::remove_var("OPBIN"),
        }
    }
}

#[derive(Debug, Clone, Default)]
pub(crate) struct PackageSeed {
    pub slug: String,
    pub uuid: String,
    pub given_name: String,
    pub family_name: String,
    pub runner: String,
    pub entrypoint: String,
    pub kind: String,
    pub transport: String,
    pub architectures: Vec<String>,
    pub has_dist: bool,
    pub has_source: bool,
    pub aliases: Vec<String>,
}

pub(crate) fn package_arch_dir() -> String {
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

pub(crate) fn file_url(path: &Path) -> String {
    let canonical = fs::canonicalize(path).unwrap_or_else(|_| path.to_path_buf());
    if let Ok(url) = reqwest::Url::from_file_path(&canonical) {
        return url.to_string();
    }

    format!(
        "file://{}",
        canonical
            .to_string_lossy()
            .replace('\\', "/")
            .trim_start_matches('/')
    )
}

pub(crate) fn compiled_fixture_holon_binary() -> PathBuf {
    static BIN: OnceLock<PathBuf> = OnceLock::new();
    BIN.get_or_init(|| {
        let root = fixture_root();
        let target_dir = root.join("target");
        write_fixture_holon_crate(&root);

        let output = Command::new("cargo")
            .arg("build")
            .arg("--manifest-path")
            .arg(root.join("Cargo.toml"))
            .arg("--target-dir")
            .arg(&target_dir)
            .output()
            .expect("failed to build fixture holon binary");

        if !output.status.success() {
            panic!(
                "cargo build for fixture holon failed:\nstdout:\n{}\nstderr:\n{}",
                String::from_utf8_lossy(&output.stdout),
                String::from_utf8_lossy(&output.stderr)
            );
        }

        let binary = target_dir.join("debug").join(fixture_binary_name());
        assert!(
            binary.is_file(),
            "missing fixture binary {}",
            binary.display()
        );
        binary
    })
    .clone()
}

fn fixture_root() -> PathBuf {
    if let Ok(path) = env::var("HOLONS_RUST_FIXTURE_ROOT") {
        let trimmed = path.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    env::temp_dir().join("holons-rust-test-fixture")
}

fn fixture_binary_name() -> &'static str {
    if cfg!(windows) {
        "holons-test-fixture.exe"
    } else {
        "holons-test-fixture"
    }
}

pub(crate) fn write_package_holon(
    directory: &Path,
    seed: &PackageSeed,
    with_holon_json: bool,
    with_binary: bool,
) {
    fs::create_dir_all(directory).unwrap();

    let entrypoint = if seed.entrypoint.trim().is_empty() {
        seed.slug.clone()
    } else {
        seed.entrypoint.clone()
    };
    let runner = if seed.runner.trim().is_empty() {
        "rust".to_string()
    } else {
        seed.runner.clone()
    };
    let kind = if seed.kind.trim().is_empty() {
        "native".to_string()
    } else {
        seed.kind.clone()
    };
    let transport = if seed.transport.trim().is_empty() {
        "stdio".to_string()
    } else {
        seed.transport.clone()
    };
    let architectures = if seed.architectures.is_empty() {
        if with_binary {
            vec![package_arch_dir()]
        } else {
            Vec::new()
        }
    } else {
        seed.architectures.clone()
    };

    if with_binary {
        let binary_path = directory
            .join("bin")
            .join(package_arch_dir())
            .join(Path::new(&entrypoint).file_name().unwrap());
        fs::create_dir_all(binary_path.parent().unwrap()).unwrap();
        fs::copy(compiled_fixture_holon_binary(), &binary_path).unwrap();
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;

            let mut perms = fs::metadata(&binary_path).unwrap().permissions();
            perms.set_mode(0o755);
            fs::set_permissions(&binary_path, perms).unwrap();
        }
    }

    if with_holon_json {
        let payload = serde_json::json!({
            "schema": "holon-package/v1",
            "slug": seed.slug,
            "uuid": seed.uuid,
            "identity": {
                "given_name": seed.given_name,
                "family_name": seed.family_name,
                "aliases": seed.aliases,
            },
            "lang": "rust",
            "runner": runner,
            "status": "draft",
            "kind": kind,
            "transport": transport,
            "entrypoint": entrypoint,
            "architectures": architectures,
            "has_dist": seed.has_dist,
            "has_source": seed.has_source,
        });
        fs::write(
            directory.join(".holon.json"),
            serde_json::to_vec_pretty(&payload)
                .map(|mut data| {
                    data.push(b'\n');
                    data
                })
                .unwrap(),
        )
        .unwrap();
    }
}

fn write_fixture_holon_crate(root: &Path) {
    fs::create_dir_all(root.join("src")).unwrap();
    fs::write(
        root.join("Cargo.toml"),
        format!(
            "[package]\nname = \"holons-test-fixture\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nholons = {{ path = {:?} }}\ntokio = {{ version = \"1\", features = [\"macros\", \"rt-multi-thread\"] }}\ntonic = {{ version = \"0.12\", features = [\"transport\"] }}\ntower-service = \"0.3\"\n",
            PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        ),
    )
    .unwrap();
    fs::write(
        root.join("src").join("describe_generated.rs"),
        r#"use holons::gen::holons::v1::{
    holon_manifest::Identity,
    DescribeResponse, HolonManifest, MethodDoc, ServiceDoc,
};

pub fn static_describe_response() -> DescribeResponse {
    DescribeResponse {
        manifest: Some(HolonManifest {
            identity: Some(Identity {
                schema: "holon/v1".to_string(),
                uuid: "fixture-holon-0000".to_string(),
                given_name: "Fixture".to_string(),
                family_name: "Holon".to_string(),
                motto: "Serves Describe from tests.".to_string(),
                composer: "rust-holons-tests".to_string(),
                status: "draft".to_string(),
                born: "2026-03-31".to_string(),
                version: "0.1.0".to_string(),
                aliases: vec!["fixture".to_string()],
            }),
            description: String::new(),
            lang: "rust".to_string(),
            skills: vec![],
            contract: None,
            kind: "native".to_string(),
            platforms: vec![],
            transport: "stdio".to_string(),
            build: None,
            requires: None,
            artifacts: None,
            sequences: vec![],
            guide: String::new(),
            session_visibility: 0,
            session_visibility_overrides: vec![],
        }),
        services: vec![ServiceDoc {
            name: "test.v1.Noop".to_string(),
            description: "Fixture Describe test service.".to_string(),
            methods: vec![MethodDoc {
                name: "Ping".to_string(),
                description: "Returns unimplemented.".to_string(),
                input_type: String::new(),
                output_type: String::new(),
                input_fields: vec![],
                output_fields: vec![],
                client_streaming: false,
                server_streaming: false,
                example_input: String::new(),
            }],
        }],
    }
}
"#,
    )
    .unwrap();
    fs::write(
        root.join("src").join("main.rs"),
        r#"use std::convert::Infallible;
use std::task::{Context, Poll};

use holons::describe;
use tonic::body::{empty_body, BoxBody};
use tonic::codegen::http::{Request, Response};
use tonic::server::NamedService;
use tower_service::Service;

mod describe_generated;

#[derive(Clone, Default)]
struct UnimplementedService;

impl NamedService for UnimplementedService {
    const NAME: &'static str = "test.v1.Noop";
}

impl Service<Request<BoxBody>> for UnimplementedService {
    type Response = Response<BoxBody>;
    type Error = Infallible;
    type Future = std::future::Ready<Result<Self::Response, Self::Error>>;

    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<Result<(), Self::Error>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, _req: Request<BoxBody>) -> Self::Future {
        let mut response = Response::new(empty_body());
        let headers = response.headers_mut();
        headers.insert(
            tonic::Status::GRPC_STATUS,
            (tonic::Code::Unimplemented as i32).into(),
        );
        headers.insert(
            tonic::codegen::http::header::CONTENT_TYPE,
            tonic::codegen::http::HeaderValue::from_static("application/grpc"),
        );
        std::future::ready(Ok(response))
    }
}

#[tokio::main(flavor = "multi_thread")]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let parsed = holons::serve::parse_options(&args);
    describe::use_static_response(describe_generated::static_describe_response());
    holons::serve::run_single_with_options(
        &parsed.listen_uri,
        UnimplementedService,
        holons::serve::RunOptions::default(),
    )
    .await?;
    Ok(())
}
"#,
    )
    .unwrap();
}
