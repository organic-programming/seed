use std::fs;
use std::path::PathBuf;
use std::process::Command;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let out_dir = PathBuf::from("gen/rust/greeting/v1");
    fs::create_dir_all(&out_dir)?;

    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .out_dir(&out_dir)
        .file_descriptor_set_path(out_dir.join("greeting_descriptor.bin"))
        .compile_protos(&["../../_protos/v1/greeting.proto"], &["../../_protos"])?;

    let holon_descriptor = PathBuf::from(
        std::env::var("OUT_DIR").expect("Cargo should always provide OUT_DIR for build scripts"),
    )
    .join("holon_descriptor.bin");

    let status = Command::new("protoc")
        .args([
            "--proto_path=api",
            "--proto_path=../../_protos",
            "--proto_path=../../../_protos",
            "v1/holon.proto",
            &format!("--descriptor_set_out={}", holon_descriptor.display()),
        ])
        .status()?;

    if !status.success() {
        return Err("failed to validate api/v1/holon.proto".into());
    }

    println!("cargo:rerun-if-changed=api/v1/holon.proto");
    println!("cargo:rerun-if-changed=../../_protos/v1/greeting.proto");
    println!("cargo:rerun-if-changed=../../../_protos/holons/v1/manifest.proto");

    Ok(())
}
