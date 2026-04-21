fn main() -> Result<(), Box<dyn std::error::Error>> {
    let out_dir = std::path::PathBuf::from("src/gen");
    let proto_root = shared_proto_root()?;
    let protos = [
        proto_root.join("holons/v1/manifest.proto"),
        proto_root.join("holons/v1/describe.proto"),
        proto_root.join("holons/v1/coax.proto"),
    ];
    std::fs::create_dir_all(&out_dir)?;

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .out_dir(&out_dir)
        .compile_protos(&protos, &[proto_root.clone()])?;

    println!(
        "cargo:rerun-if-changed={}",
        proto_root.join("holons/v1/manifest.proto").display()
    );
    println!(
        "cargo:rerun-if-changed={}",
        proto_root.join("holons/v1/describe.proto").display()
    );
    println!(
        "cargo:rerun-if-changed={}",
        proto_root.join("holons/v1/coax.proto").display()
    );
    Ok(())
}

fn shared_proto_root() -> Result<std::path::PathBuf, Box<dyn std::error::Error>> {
    let manifest_dir = std::path::PathBuf::from(std::env::var("CARGO_MANIFEST_DIR")?);
    let candidates = [
        manifest_dir.join("../../_protos"),
        manifest_dir.join("../../holons/grace-op/_protos"),
    ];

    for candidate in candidates {
        if candidate.join("holons/v1/manifest.proto").is_file() {
            return Ok(candidate);
        }
    }

    Err("unable to locate shared holons proto root".into())
}
