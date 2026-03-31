use hyper_util::rt::TokioIo;
use std::path::Path;
use std::pin::Pin;
use std::process::Stdio;
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll};
use std::time::{Duration, Instant};
use tokio::io::{AsyncRead, AsyncWrite, ReadBuf};
use tokio::process::{Child, ChildStdin, ChildStdout, Command};
use tonic::codegen::http::Uri;
use tonic::transport::{Channel, Endpoint};
use tower_service::Service;

pub(crate) struct ChildStdioTransport {
    reader: ChildStdout,
    writer: ChildStdin,
}

struct StdioConnector {
    transport: Arc<Mutex<Option<ChildStdioTransport>>>,
}

impl AsyncRead for ChildStdioTransport {
    fn poll_read(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.reader).poll_read(cx, buf)
    }
}

impl AsyncWrite for ChildStdioTransport {
    fn poll_write(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<std::io::Result<usize>> {
        Pin::new(&mut self.writer).poll_write(cx, buf)
    }

    fn poll_flush(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.writer).poll_flush(cx)
    }

    fn poll_shutdown(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Pin::new(&mut self.writer).poll_shutdown(cx)
    }
}

impl Service<Uri> for StdioConnector {
    type Response = TokioIo<ChildStdioTransport>;
    type Error = std::io::Error;
    type Future = Pin<
        Box<dyn std::future::Future<Output = std::io::Result<TokioIo<ChildStdioTransport>>> + Send>,
    >;

    fn poll_ready(&mut self, _cx: &mut Context<'_>) -> Poll<std::io::Result<()>> {
        Poll::Ready(Ok(()))
    }

    fn call(&mut self, _uri: Uri) -> Self::Future {
        let transport = self.transport.lock().unwrap().take();
        Box::pin(async move {
            transport.map(TokioIo::new).ok_or_else(|| {
                std::io::Error::new(
                    std::io::ErrorKind::BrokenPipe,
                    "stdio transport already consumed",
                )
            })
        })
    }
}

pub(crate) fn start_stdio_command(
    command_path: &Path,
    args: &[String],
    cwd: Option<&Path>,
) -> Result<(ChildStdioTransport, Child), String> {
    let mut command = Command::new(command_path);
    #[cfg(unix)]
    command.process_group(0);
    if let Some(cwd) = cwd {
        command.current_dir(cwd);
    }
    command
        .args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null());

    let mut child = command.spawn().map_err(|err| err.to_string())?;
    let stdin = child
        .stdin
        .take()
        .ok_or_else(|| "failed to capture child stdin".to_string())?;
    let stdout = child
        .stdout
        .take()
        .ok_or_else(|| "failed to capture child stdout".to_string())?;

    Ok((
        ChildStdioTransport {
            reader: stdout,
            writer: stdin,
        },
        child,
    ))
}

pub(crate) async fn connect_stdio_transport(
    transport: ChildStdioTransport,
    timeout: u32,
) -> Result<Channel, String> {
    endpoint_with_timeout("http://127.0.0.1:50051", timeout)?
        .connect_with_connector(StdioConnector {
            transport: Arc::new(Mutex::new(Some(transport))),
        })
        .await
        .map_err(|err| err.to_string())
}

pub(crate) fn endpoint_with_timeout(target: &str, timeout: u32) -> Result<Endpoint, String> {
    let endpoint = Endpoint::from_shared(target.to_string()).map_err(|err| err.to_string())?;
    if timeout == 0 {
        return Ok(endpoint);
    }
    let duration = Duration::from_millis(timeout as u64);
    Ok(endpoint.connect_timeout(duration).timeout(duration))
}

pub(crate) fn stop_child(child: &mut Child) -> Result<(), String> {
    if child.try_wait().map_err(|err| err.to_string())?.is_some() {
        return Ok(());
    }

    if let Some(pid) = child.id() {
        let _ = send_sigterm(pid);
        let deadline = Instant::now() + Duration::from_secs(2);
        while Instant::now() < deadline {
            if child.try_wait().map_err(|err| err.to_string())?.is_some() {
                return Ok(());
            }
            std::thread::sleep(Duration::from_millis(50));
        }
    }

    if let Some(pid) = child.id() {
        let _ = send_sigkill(pid);
    }
    let _ = child.start_kill();

    let deadline = Instant::now() + Duration::from_secs(2);
    while Instant::now() < deadline {
        if child.try_wait().map_err(|err| err.to_string())?.is_some() {
            return Ok(());
        }
        std::thread::sleep(Duration::from_millis(50));
    }

    Ok(())
}

#[cfg(unix)]
fn send_sigterm(pid: u32) -> Result<(), String> {
    send_signal(pid, libc::SIGTERM)
}

#[cfg(not(unix))]
fn send_sigterm(pid: u32) -> Result<(), String> {
    let _ = pid;
    Ok(())
}

#[cfg(unix)]
fn send_sigkill(pid: u32) -> Result<(), String> {
    send_signal(pid, libc::SIGKILL)
}

#[cfg(not(unix))]
fn send_sigkill(pid: u32) -> Result<(), String> {
    let _ = pid;
    Ok(())
}

#[cfg(unix)]
fn send_signal(pid: u32, signal: i32) -> Result<(), String> {
    let status = unsafe { libc::kill(-(pid as i32), signal) };
    if status == 0 || std::io::Error::last_os_error().raw_os_error() == Some(libc::ESRCH) {
        return Ok(());
    }
    Err(std::io::Error::last_os_error().to_string())
}
