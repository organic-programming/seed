use std::env;
use std::path::PathBuf;
use std::process::Command;

fn main() {
    let script = match resolve_script() {
        Ok(path) => path,
        Err(err) => {
            eprintln!("{}", err);
            std::process::exit(1);
        }
    };

    let status = Command::new("/bin/sh")
        .arg(script)
        .status()
        .expect("failed to run /bin/sh");
    std::process::exit(status.code().unwrap_or(1));
}

fn resolve_script() -> Result<PathBuf, Box<dyn std::error::Error>> {
    if let Ok(path) = env::var("CHARON_RUN_SCRIPT") {
        if !path.trim().is_empty() {
            return Ok(PathBuf::from(path));
        }
    }
    let exe = env::current_exe()?;
    Ok(exe.parent().unwrap().join("scripts").join("run.sh"))
}
