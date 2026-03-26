use std::env;
use std::path::PathBuf;
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
