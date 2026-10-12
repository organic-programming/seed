//! Host and child process lifetime ownership. Windows jobs close the orphan
//! window before any child is created; Unix retains its process-group behavior.

#[cfg(windows)]
mod windows;

/// Establish host ownership before starting children. Call at CLI startup when
/// a program can spawn processes before entering the SDK's serve/dial helpers.
pub fn ensure() -> Result<(), String> {
    #[cfg(windows)]
    return windows::ensure();
    #[cfg(not(windows))]
    Ok(())
}

pub(crate) fn spawn(
    command: &mut tokio::process::Command,
) -> Result<tokio::process::Child, String> {
    #[cfg(windows)]
    return windows::spawn(command);
    #[cfg(not(windows))]
    command.spawn().map_err(|err| err.to_string())
}

#[cfg(windows)]
pub(crate) use windows::{kill, terminate};
