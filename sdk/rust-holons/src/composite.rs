//! Helpers for composite holons.

use std::env;
use std::fs;
use std::io;
use std::path::{Path, PathBuf};

/// Resolve a declared member's binary relative to the calling composite's
/// own executable.
pub fn member(id: &str) -> io::Result<PathBuf> {
    member_from_executable(&env::current_exe()?, id)
}

fn member_from_executable(self_path: &Path, id: &str) -> io::Result<PathBuf> {
    let id = id.trim();
    if id.is_empty() {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "member id is required",
        ));
    }
    let bin_dir = self_path.parent().ok_or_else(|| {
        io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("executable has no parent: {}", self_path.display()),
        )
    })?;
    let member_dir = bin_dir.join("holons").join(id);
    for entry in fs::read_dir(&member_dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file() && is_executable(&path)? {
            return Ok(path);
        }
    }
    Err(io::Error::new(
        io::ErrorKind::NotFound,
        format!("no executable found in {}", member_dir.display()),
    ))
}

fn is_executable(path: &Path) -> io::Result<bool> {
    let meta = fs::metadata(path)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        Ok(meta.permissions().mode() & 0o111 != 0)
    }
    #[cfg(windows)]
    {
        Ok(path
            .extension()
            .and_then(|ext| ext.to_str())
            .map(|ext| ext.eq_ignore_ascii_case("exe"))
            .unwrap_or(false))
    }
    #[cfg(not(any(unix, windows)))]
    {
        Ok(meta.is_file())
    }
}

#[cfg(test)]
mod tests {
    use super::member_from_executable;
    use std::fs;

    #[cfg(unix)]
    use std::os::unix::fs::PermissionsExt;

    #[test]
    fn resolves_embedded_member_binary() {
        let root = tempfile::tempdir().unwrap();
        let bin_dir = root.path().join("composite.holon/bin/darwin_arm64");
        let member_dir = bin_dir.join("holons/node-a");
        fs::create_dir_all(&member_dir).unwrap();
        let self_path = bin_dir.join("composite");
        fs::write(&self_path, b"composite").unwrap();
        fs::write(member_dir.join("README.txt"), b"not executable").unwrap();
        let member = member_dir.join("node-a-bin");
        fs::write(&member, b"member").unwrap();
        #[cfg(unix)]
        fs::set_permissions(&member, fs::Permissions::from_mode(0o755)).unwrap();

        let got = member_from_executable(&self_path, "node-a").unwrap();
        assert_eq!(got, member);
    }

    #[test]
    fn rejects_empty_member_id() {
        let err = member_from_executable(std::path::Path::new("/tmp/composite"), " ")
            .expect_err("empty id should fail");
        assert_eq!(err.kind(), std::io::ErrorKind::InvalidInput);
    }
}
