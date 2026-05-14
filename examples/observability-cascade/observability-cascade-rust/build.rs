use std::fs;
use std::path::PathBuf;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let manifest_dir = PathBuf::from(std::env::var("CARGO_MANIFEST_DIR")?);
    let proto_root = shared_proto_root(&manifest_dir)?;
    let out_dir = PathBuf::from("gen/rust/observability_cascade/v1");
    fs::create_dir_all(&out_dir)?;
    let descriptor = out_dir.join("observability_cascade_descriptor.bin");
    let service_proto = proto_root.join("observability_cascade/v1/service.proto");

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .file_descriptor_set_path(&descriptor)
        .out_dir(&out_dir)
        .compile_protos(
            std::slice::from_ref(&service_proto),
            std::slice::from_ref(&proto_root),
        )?;

    println!(
        "cargo:rerun-if-changed={}",
        manifest_dir.join("api/v1/holon.proto").display()
    );
    println!("cargo:rerun-if-changed={}", service_proto.display());

    Ok(())
}

fn shared_proto_root(manifest_dir: &PathBuf) -> Result<PathBuf, Box<dyn std::error::Error>> {
    let candidates = [manifest_dir.join(".op/protos"), manifest_dir.join("../_protos")];

    for candidate in candidates {
        if candidate
            .join("observability_cascade/v1/service.proto")
            .is_file()
        {
            return Ok(candidate);
        }
    }

    Err("unable to locate shared observability cascade proto root".into())
}
