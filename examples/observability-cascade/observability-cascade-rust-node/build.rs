use std::fs;
use std::path::PathBuf;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let manifest_dir = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR")?);
    let proto_root = shared_proto_root(&manifest_dir)?;
    let out_dir = PathBuf::from("gen/rust/relay/v1");
    fs::create_dir_all(&out_dir)?;
    let relay_descriptor = out_dir.join("relay_descriptor.bin");
    let relay_proto = proto_root.join("relay/v1/relay.proto");

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .file_descriptor_set_path(&relay_descriptor)
        .out_dir(&out_dir)
        .compile_protos(
            std::slice::from_ref(&relay_proto),
            std::slice::from_ref(&proto_root),
        )?;

    println!(
        "cargo:rerun-if-changed={}",
        manifest_dir.join("api/v1/holon.proto").display()
    );
    println!("cargo:rerun-if-changed={}", relay_proto.display());
    println!(
        "cargo:rerun-if-changed={}",
        proto_root.join("holons/v1/manifest.proto").display()
    );

    Ok(())
}

fn shared_proto_root(manifest_dir: &PathBuf) -> Result<PathBuf, Box<dyn std::error::Error>> {
    let candidates = [
        manifest_dir.join("../_protos"),
        manifest_dir.join("../../../_protos"),
        manifest_dir.join(".op/protos"),
    ];

    for candidate in candidates {
        if candidate.join("relay/v1/relay.proto").is_file() {
            return Ok(candidate);
        }
    }

    Err("unable to locate shared relay proto root".into())
}
