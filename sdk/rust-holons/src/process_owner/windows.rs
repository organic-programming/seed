use std::collections::HashMap;
use std::io;
use std::mem::{size_of, zeroed};
use std::os::windows::io::{AsRawHandle, FromRawHandle, OwnedHandle};
use std::ptr::{null, null_mut};
use std::sync::{Mutex, OnceLock};
use tokio::process::{Child, Command};
use windows_sys::Win32::Foundation::{
    DuplicateHandle, DUPLICATE_SAME_ACCESS, HANDLE, INVALID_HANDLE_VALUE,
};
use windows_sys::Win32::System::Console::{GenerateConsoleCtrlEvent, CTRL_BREAK_EVENT};
use windows_sys::Win32::System::Diagnostics::ToolHelp::{
    CreateToolhelp32Snapshot, Thread32First, Thread32Next, TH32CS_SNAPTHREAD, THREADENTRY32,
};
use windows_sys::Win32::System::JobObjects::{
    AssignProcessToJobObject, CreateJobObjectW, JobObjectExtendedLimitInformation,
    SetInformationJobObject, TerminateJobObject, JOBOBJECT_EXTENDED_LIMIT_INFORMATION,
    JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
};
use windows_sys::Win32::System::Threading::{
    GetCurrentProcess, OpenThread, ResumeThread, WaitForSingleObject, CREATE_NEW_PROCESS_GROUP,
    CREATE_SUSPENDED, INFINITE, THREAD_SUSPEND_RESUME,
};

// The owner handle deliberately lives until OS process teardown: closing it
// early would kill this process too. None of the handles are inheritable.
static OWNER: OnceLock<Result<OwnedHandle, String>> = OnceLock::new();
static CHILDREN: OnceLock<Mutex<HashMap<u32, OwnedHandle>>> = OnceLock::new();

fn children() -> &'static Mutex<HashMap<u32, OwnedHandle>> {
    CHILDREN.get_or_init(|| Mutex::new(HashMap::new()))
}

pub(super) fn ensure() -> Result<(), String> {
    OWNER
        .get_or_init(|| {
            let job = create_job()?;
            if unsafe { AssignProcessToJobObject(job.as_raw_handle(), GetCurrentProcess()) } == 0 {
                return Err(error("assign owner job"));
            }
            Ok(job)
        })
        .as_ref()
        .map(|_| ())
        .map_err(Clone::clone)
}

fn create_job() -> Result<OwnedHandle, String> {
    let raw = unsafe { CreateJobObjectW(null(), null()) };
    if raw.is_null() {
        return Err(error("create process job"));
    }
    let job = unsafe { OwnedHandle::from_raw_handle(raw) };
    let mut limits: JOBOBJECT_EXTENDED_LIMIT_INFORMATION = unsafe { zeroed() };
    limits.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
    if unsafe {
        SetInformationJobObject(
            raw,
            JobObjectExtendedLimitInformation,
            &limits as *const _ as *const _,
            size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
        )
    } == 0
    {
        return Err(error("configure process job"));
    }
    Ok(job)
}

pub(super) fn spawn(command: &mut Command) -> Result<Child, String> {
    ensure()?;
    let job = create_job()?;
    command
        .creation_flags(CREATE_SUSPENDED | CREATE_NEW_PROCESS_GROUP)
        .kill_on_drop(true);
    let mut child = command.spawn().map_err(|err| err.to_string())?;
    // This child already inherited the owner job while suspended. If the host
    // crashes before assigning its private child job, the OS still kills it.
    let pid = child.id().ok_or("spawned child has no process ID")?;
    let process = child
        .raw_handle()
        .ok_or("spawned child has no process handle")?;
    if unsafe { AssignProcessToJobObject(job.as_raw_handle(), process) } == 0 {
        let message = error("assign child job");
        let _ = child.start_kill();
        return Err(message);
    }
    let mut duplicate: HANDLE = null_mut();
    if unsafe {
        DuplicateHandle(
            GetCurrentProcess(),
            process,
            GetCurrentProcess(),
            &mut duplicate,
            0,
            0,
            DUPLICATE_SAME_ACCESS,
        )
    } == 0
    {
        return Err(error("duplicate child process handle"));
    }
    let process = unsafe { OwnedHandle::from_raw_handle(duplicate) };
    children().lock().unwrap().insert(pid, job);
    // Keep job cleanup independent of Child::wait/try_wait and drop. Closing
    // the job also releases any pipes held open by surviving grandchildren.
    if let Err(err) = std::thread::Builder::new()
        .name("holon-child-job".into())
        .spawn(move || {
            unsafe { WaitForSingleObject(process.as_raw_handle(), INFINITE) };
            children().lock().unwrap().remove(&pid);
        })
    {
        children().lock().unwrap().remove(&pid);
        return Err(err.to_string());
    }
    if let Err(err) = resume_initial_thread(pid) {
        children().lock().unwrap().remove(&pid);
        return Err(err);
    }
    Ok(child)
}

fn resume_initial_thread(pid: u32) -> Result<(), String> {
    let snapshot = unsafe { CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0) };
    if snapshot == INVALID_HANDLE_VALUE {
        return Err(error("snapshot child threads"));
    }
    let snapshot = unsafe { OwnedHandle::from_raw_handle(snapshot) };
    let mut entry: THREADENTRY32 = unsafe { zeroed() };
    entry.dwSize = size_of::<THREADENTRY32>() as u32;
    let mut found = unsafe { Thread32First(snapshot.as_raw_handle(), &mut entry) };
    while found != 0 {
        if entry.th32OwnerProcessID == pid {
            let thread = unsafe { OpenThread(THREAD_SUSPEND_RESUME, 0, entry.th32ThreadID) };
            if thread.is_null() {
                return Err(error("open suspended child thread"));
            }
            let thread = unsafe { OwnedHandle::from_raw_handle(thread) };
            if unsafe { ResumeThread(thread.as_raw_handle()) } == u32::MAX {
                return Err(error("resume child thread"));
            }
            return Ok(());
        }
        found = unsafe { Thread32Next(snapshot.as_raw_handle(), &mut entry) };
    }
    Err(format!(
        "initial thread for suspended process {pid} not found"
    ))
}

pub(crate) fn terminate(pid: u32) -> Result<(), String> {
    let jobs = children().lock().unwrap();
    if !jobs.contains_key(&pid) {
        return Ok(());
    }
    if unsafe { GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT, pid) } == 0 {
        return Err(error("request child console shutdown"));
    }
    Ok(())
}

pub(crate) fn kill(pid: u32) -> Result<(), String> {
    let jobs = children().lock().unwrap();
    if let Some(job) = jobs.get(&pid) {
        if unsafe { TerminateJobObject(job.as_raw_handle(), 1) } == 0 {
            return Err(error("terminate child job"));
        }
    }
    Ok(())
}

fn error(operation: &str) -> String {
    format!("{operation}: {}", io::Error::last_os_error())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;
    use std::process::Command as StdCommand;
    use std::time::{Duration, Instant};
    use windows_sys::Win32::Foundation::WAIT_OBJECT_0;
    use windows_sys::Win32::System::Threading::{OpenProcess, PROCESS_SYNCHRONIZE};

    fn helper(role: &str, root: &Path) -> StdCommand {
        let mut command = StdCommand::new(std::env::current_exe().unwrap());
        command
            .args([
                "--exact",
                "process_owner::windows::tests::process_helper",
                "--nocapture",
            ])
            .env("HOLONS_JOB_TEST_ROLE", role)
            .env("HOLONS_JOB_TEST_ROOT", root);
        command
    }

    #[test]
    fn process_helper() {
        let Ok(role) = std::env::var("HOLONS_JOB_TEST_ROLE") else {
            return;
        };
        let root = std::path::PathBuf::from(std::env::var_os("HOLONS_JOB_TEST_ROOT").unwrap());
        if role == "owner" || role == "branch" {
            ensure().unwrap();
            ensure().unwrap();
        }
        if role != "leaf" {
            let next = if role == "owner" { "branch" } else { "leaf" };
            let _child = helper(next, &root).spawn().unwrap();
        }
        std::fs::write(
            root.join(format!("{role}.pid")),
            std::process::id().to_string(),
        )
        .unwrap();
        loop {
            if (role == "owner" || role == "unowned") && root.join("exit").exists() {
                std::process::exit(0);
            }
            std::thread::sleep(Duration::from_millis(10));
        }
    }

    fn descendant(root: &Path, role: &str) -> OwnedHandle {
        let deadline = Instant::now() + Duration::from_secs(10);
        loop {
            if let Ok(text) = std::fs::read_to_string(root.join(format!("{role}.pid"))) {
                if let Ok(pid) = text.parse::<u32>() {
                    let raw = unsafe { OpenProcess(PROCESS_SYNCHRONIZE, 0, pid) };
                    assert!(!raw.is_null(), "{}", error("open descendant"));
                    return unsafe { OwnedHandle::from_raw_handle(raw) };
                }
            }
            assert!(Instant::now() < deadline, "{role} did not start");
            std::thread::sleep(Duration::from_millis(10));
        }
    }

    fn assert_exited(process: &OwnedHandle) {
        assert_eq!(
            unsafe { WaitForSingleObject(process.as_raw_handle(), 5000) },
            WAIT_OBJECT_0,
            "descendant survived process owner exit"
        );
    }

    #[test]
    fn owner_exit_terminates_nested_descendants() {
        for forced in [false, true] {
            let root = tempfile::tempdir().unwrap();
            let mut child = helper("owner", root.path()).spawn().unwrap();
            let branch = descendant(root.path(), "branch");
            let leaf = descendant(root.path(), "leaf");
            if forced {
                child.kill().unwrap();
            } else {
                std::fs::write(root.path().join("exit"), []).unwrap();
            }
            child.wait().unwrap();
            assert_exited(&branch);
            assert_exited(&leaf);
        }
    }

    #[tokio::test]
    async fn child_job_owns_uncooperative_grandchild() {
        for forced in [false, true] {
            let root = tempfile::tempdir().unwrap();
            let mut command: Command = helper("unowned", root.path()).into();
            let mut child = spawn(&mut command).unwrap();
            let leaf = descendant(root.path(), "leaf");
            if forced {
                kill(child.id().unwrap()).unwrap();
            } else {
                std::fs::write(root.path().join("exit"), []).unwrap();
            }
            child.wait().await.unwrap();
            assert_exited(&leaf);
        }
    }
}
