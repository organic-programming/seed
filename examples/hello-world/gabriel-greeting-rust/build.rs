use std::fs;
use std::path::PathBuf;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let manifest_dir = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR")?);
    let proto_root = shared_proto_root(&manifest_dir)?;
    let out_dir = PathBuf::from("gen/rust/greeting/v1");
    fs::create_dir_all(&out_dir)?;
    let greeting_descriptor = out_dir.join("greeting_descriptor.bin");
    let greeting_proto = proto_root.join("v1/greeting.proto");

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .file_descriptor_set_path(&greeting_descriptor)
        .out_dir(&out_dir)
        .compile_protos(&[greeting_proto.clone()], &[proto_root.clone()])?;

    println!(
        "cargo:rerun-if-changed={}",
        manifest_dir.join("api/v1/holon.proto").display()
    );
    println!("cargo:rerun-if-changed={}", greeting_proto.display());
    println!(
        "cargo:rerun-if-changed={}",
        proto_root.join("holons/v1/manifest.proto").display()
    );

    Ok(())
}

fn shared_proto_root(manifest_dir: &PathBuf) -> Result<PathBuf, Box<dyn std::error::Error>> {
    let candidates = [
        manifest_dir.join(".op/protos"),
        manifest_dir.join("../../../_protos"),
    ];

    for candidate in candidates {
        if candidate.join("holons/v1/manifest.proto").is_file()
            && candidate.join("v1/greeting.proto").is_file()
        {
            return Ok(candidate);
        }
    }

    Err("unable to locate shared holons proto root".into())
}
