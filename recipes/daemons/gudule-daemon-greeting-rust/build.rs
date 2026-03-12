fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(true)
        .build_client(false)
        .file_descriptor_set_path("greeting_descriptor.bin")
        .compile_protos(&["proto/greeting.proto"], &["proto"])?;
    Ok(())
}
