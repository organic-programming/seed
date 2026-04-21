use std::fs;
use std::path::PathBuf;

#[test]
fn test_echo_scripts_exist() {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let echo_client = manifest_dir.join("bin/echo-client");
    let echo_server = manifest_dir.join("bin/echo-server");
    let holon_rpc_client = manifest_dir.join("bin/holon-rpc-client");
    let holon_rpc_server = manifest_dir.join("bin/holon-rpc-server");

    assert!(echo_client.is_file());
    assert!(echo_server.is_file());
    assert!(holon_rpc_client.is_file());
    assert!(holon_rpc_server.is_file());
}

#[test]
fn test_go_helper_files_exist_for_wrappers() {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let echo_server_helper = manifest_dir.join("cmd/echo-server-go/main.go");
    let holon_rpc_client_helper = manifest_dir.join("cmd/holon-rpc-client-go/main.go");

    assert!(echo_server_helper.is_file());
    assert!(holon_rpc_client_helper.is_file());
}

#[cfg(unix)]
#[test]
fn test_echo_scripts_are_executable() {
    use std::os::unix::fs::PermissionsExt;

    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let echo_client = manifest_dir.join("bin/echo-client");
    let echo_server = manifest_dir.join("bin/echo-server");
    let holon_rpc_client = manifest_dir.join("bin/holon-rpc-client");
    let holon_rpc_server = manifest_dir.join("bin/holon-rpc-server");

    let client_mode = fs::metadata(echo_client).unwrap().permissions().mode();
    let server_mode = fs::metadata(echo_server).unwrap().permissions().mode();
    let holon_rpc_client_mode = fs::metadata(holon_rpc_client).unwrap().permissions().mode();
    let holon_rpc_server_mode = fs::metadata(holon_rpc_server).unwrap().permissions().mode();

    assert_ne!(client_mode & 0o111, 0);
    assert_ne!(server_mode & 0o111, 0);
    assert_ne!(holon_rpc_client_mode & 0o111, 0);
    assert_ne!(holon_rpc_server_mode & 0o111, 0);
}
