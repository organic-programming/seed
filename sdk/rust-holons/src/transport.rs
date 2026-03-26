//! URI-based listener factory for gRPC servers.
//!
//! Supported transport URIs:
//!   - `tcp://<host>:<port>` — TCP socket (default: `tcp://:9090`)
//!   - `unix://<path>`       — Unix domain socket
//!   - `stdio://`            — stdin/stdout pipe
//!   - `ws://<host>:<port>`  — WebSocket URI surface
//!   - `wss://<host>:<port>` — WebSocket-over-TLS URI surface

use std::io;
use std::net::SocketAddr;
use std::pin::Pin;
use std::task::{Context, Poll};

use tokio::io::{AsyncRead, AsyncWrite, ReadBuf, Stdin, Stdout};
#[cfg(not(unix))]
use tokio::net::{TcpListener, TcpStream};
#[cfg(unix)]
use tokio::net::{TcpListener, TcpStream, UnixListener, UnixStream};

/// Default transport URI when --listen is omitted.
pub const DEFAULT_URI: &str = "tcp://:9090";

/// Parsed transport URI.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ParsedURI {
    pub raw: String,
    pub scheme: String,
    pub host: Option<String>,
    pub port: Option<u16>,
    pub path: Option<String>,
    pub secure: bool,
}

/// Listener variants returned by [`listen`].
pub enum Listener {
    Tcp(TcpListener),
    #[cfg(unix)]
    Unix(UnixListener),
    Stdio,
    Ws(WSListener),
}

/// Lightweight ws/wss listener metadata.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct WSListener {
    pub host: String,
    pub port: u16,
    pub path: String,
    pub secure: bool,
}

/// Bidirectional stdio transport: reads from stdin and writes to stdout.
pub struct StdioTransport<R = Stdin, W = Stdout> {
    reader: R,
    writer: W,
}

impl<R, W> StdioTransport<R, W> {
    pub fn new(reader: R, writer: W) -> Self {
        Self { reader, writer }
    }
}

impl<R, W> AsyncRead for StdioTransport<R, W>
where
    R: AsyncRead + Unpin,
    W: Unpin,
{
    fn poll_read(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &mut ReadBuf<'_>,
    ) -> Poll<io::Result<()>> {
        let this = self.get_mut();
        Pin::new(&mut this.reader).poll_read(cx, buf)
    }
}

impl<R, W> AsyncWrite for StdioTransport<R, W>
where
    R: Unpin,
    W: AsyncWrite + Unpin,
{
    fn poll_write(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<io::Result<usize>> {
        let this = self.get_mut();
        Pin::new(&mut this.writer).poll_write(cx, buf)
    }

    fn poll_flush(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        let this = self.get_mut();
        Pin::new(&mut this.writer).poll_flush(cx)
    }

    fn poll_shutdown(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        let this = self.get_mut();
        Pin::new(&mut this.writer).poll_shutdown(cx)
    }
}

/// Parse a transport URI and bind/create the appropriate listener.
pub async fn listen(uri: &str) -> io::Result<Listener> {
    let parsed = parse_uri(uri).map_err(|err| io::Error::new(io::ErrorKind::InvalidInput, err))?;

    match parsed.scheme.as_str() {
        "tcp" => {
            let host = parsed.host.unwrap_or_else(|| "0.0.0.0".to_string());
            let port = parsed.port.unwrap_or(9090);
            let lis = TcpListener::bind(format!("{}:{}", host, port)).await?;
            Ok(Listener::Tcp(lis))
        }
        "unix" => listen_unix(parsed.path.unwrap_or_default()),
        "stdio" => Ok(Listener::Stdio),
        "ws" | "wss" => Ok(Listener::Ws(WSListener {
            host: parsed.host.unwrap_or_else(|| "0.0.0.0".to_string()),
            port: parsed.port.unwrap_or(if parsed.secure { 443 } else { 80 }),
            path: parsed.path.unwrap_or_else(|| "/grpc".to_string()),
            secure: parsed.secure,
        })),
        _ => Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("unsupported transport URI: {uri:?}"),
        )),
    }
}

#[cfg(unix)]
fn listen_unix(path: String) -> io::Result<Listener> {
    let _ = std::fs::remove_file(&path);
    let lis = UnixListener::bind(path)?;
    Ok(Listener::Unix(lis))
}

#[cfg(not(unix))]
fn listen_unix(_path: String) -> io::Result<Listener> {
    Err(io::Error::new(
        io::ErrorKind::Unsupported,
        "unix:// transport is only supported on unix platforms",
    ))
}

/// Create a stdio transport that can be passed to async gRPC adapters.
pub fn listen_stdio() -> io::Result<StdioTransport> {
    Ok(StdioTransport::new(tokio::io::stdin(), tokio::io::stdout()))
}

/// Dial a remote TCP listener from a `tcp://` URI.
pub async fn dial_tcp(uri: &str) -> io::Result<TcpStream> {
    let parsed = parse_uri(uri).map_err(|err| io::Error::new(io::ErrorKind::InvalidInput, err))?;
    if parsed.scheme != "tcp" {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("dial_tcp expects tcp:// URI, got {uri:?}"),
        ));
    }
    let host = parsed.host.unwrap_or_else(|| "127.0.0.1".to_string());
    let port = parsed.port.unwrap_or(9090);
    TcpStream::connect(format!("{host}:{port}")).await
}

/// Dial a remote Unix-domain listener from a `unix://` URI.
#[cfg(unix)]
pub async fn dial_unix(uri: &str) -> io::Result<UnixStream> {
    let parsed = parse_uri(uri).map_err(|err| io::Error::new(io::ErrorKind::InvalidInput, err))?;
    if parsed.scheme != "unix" {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("dial_unix expects unix:// URI, got {uri:?}"),
        ));
    }
    let path = parsed.path.unwrap_or_default();
    if path.is_empty() {
        return Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!("unix URI requires a path: {uri:?}"),
        ));
    }
    UnixStream::connect(path).await
}

/// Dial a remote Unix-domain listener from a `unix://` URI.
#[cfg(not(unix))]
pub async fn dial_unix(uri: &str) -> io::Result<()> {
    let _ = uri;
    Err(io::Error::new(
        io::ErrorKind::Unsupported,
        "dial_unix is only supported on unix platforms",
    ))
}

/// Extract the transport scheme from a URI.
pub fn scheme(uri: &str) -> &str {
    uri.find("://").map_or(uri, |i| &uri[..i])
}

/// Parse a URI into a normalized structure.
pub fn parse_uri(uri: &str) -> Result<ParsedURI, String> {
    let s = scheme(uri);

    match s {
        "tcp" => {
            let addr = uri
                .strip_prefix("tcp://")
                .ok_or_else(|| format!("invalid tcp URI: {uri:?}"))?;
            let (host, port) = split_host_port(addr, 9090)?;
            Ok(ParsedURI {
                raw: uri.to_string(),
                scheme: "tcp".to_string(),
                host: Some(host),
                port: Some(port),
                path: None,
                secure: false,
            })
        }
        "unix" => {
            let path = uri
                .strip_prefix("unix://")
                .ok_or_else(|| format!("invalid unix URI: {uri:?}"))?;
            if path.is_empty() {
                return Err(format!("invalid unix URI: {uri:?}"));
            }
            Ok(ParsedURI {
                raw: uri.to_string(),
                scheme: "unix".to_string(),
                host: None,
                port: None,
                path: Some(path.to_string()),
                secure: false,
            })
        }
        "stdio" => Ok(ParsedURI {
            raw: "stdio://".to_string(),
            scheme: "stdio".to_string(),
            host: None,
            port: None,
            path: None,
            secure: false,
        }),
        "ws" | "wss" => {
            let secure = s == "wss";
            let trimmed = uri
                .strip_prefix(if secure { "wss://" } else { "ws://" })
                .ok_or_else(|| format!("invalid ws URI: {uri:?}"))?;

            let (addr, path) = if let Some((a, p)) = trimmed.split_once('/') {
                (a, format!("/{p}"))
            } else {
                (trimmed, "/grpc".to_string())
            };

            let (host, port) = split_host_port(addr, if secure { 443 } else { 80 })?;

            Ok(ParsedURI {
                raw: uri.to_string(),
                scheme: s.to_string(),
                host: Some(host),
                port: Some(port),
                path: Some(path),
                secure,
            })
        }
        _ => Err(format!("unsupported transport URI: {uri:?}")),
    }
}

/// Retrieve the local address of a [`Listener::Tcp`].
pub fn local_addr(lis: &Listener) -> Option<SocketAddr> {
    match lis {
        Listener::Tcp(l) => l.local_addr().ok(),
        _ => None,
    }
}

fn split_host_port(value: &str, default_port: u16) -> Result<(String, u16), String> {
    if value.is_empty() {
        return Ok(("0.0.0.0".to_string(), default_port));
    }

    if let Some((host, port_s)) = value.rsplit_once(':') {
        let host = if host.is_empty() { "0.0.0.0" } else { host };
        let port = if port_s.is_empty() {
            default_port
        } else {
            port_s
                .parse::<u16>()
                .map_err(|_| format!("invalid port in URI: {value:?}"))?
        };
        Ok((host.to_string(), port))
    } else {
        Ok((value.to_string(), default_port))
    }
}

#[cfg(test)]
mod tests {
    use std::io;
    #[cfg(unix)]
    use std::time::{Duration, SystemTime, UNIX_EPOCH};
    use tokio::io::{AsyncReadExt, AsyncWriteExt};

    use super::*;

    #[test]
    fn test_scheme() {
        assert_eq!(scheme("tcp://:9090"), "tcp");
        assert_eq!(scheme("unix:///tmp/x.sock"), "unix");
        assert_eq!(scheme("stdio://"), "stdio");
        assert_eq!(scheme("ws://host:8080"), "ws");
        assert_eq!(scheme("wss://host:443"), "wss");
    }

    #[test]
    fn test_default_uri() {
        assert_eq!(DEFAULT_URI, "tcp://:9090");
    }

    #[test]
    fn test_parse_uri_wss() {
        let parsed = parse_uri("wss://example.com:8443").unwrap();
        assert_eq!(parsed.scheme, "wss");
        assert_eq!(parsed.host.as_deref(), Some("example.com"));
        assert_eq!(parsed.port, Some(8443));
        assert_eq!(parsed.path.as_deref(), Some("/grpc"));
        assert!(parsed.secure);
    }

    #[tokio::test]
    async fn test_tcp_listen() {
        let lis = match listen("tcp://127.0.0.1:0").await {
            Ok(v) => v,
            Err(err) if err.kind() == io::ErrorKind::PermissionDenied => return,
            Err(err) => panic!("tcp listen failed: {err}"),
        };
        match &lis {
            Listener::Tcp(l) => {
                let addr = l.local_addr().unwrap();
                assert!(addr.port() > 0);
            }
            _ => panic!("expected Tcp listener"),
        }
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn test_unix_listen() {
        let path = "/tmp/holons_test_rust.sock";
        let lis = match listen(&format!("unix://{}", path)).await {
            Ok(v) => v,
            Err(err) if err.kind() == io::ErrorKind::PermissionDenied => return,
            Err(err) => panic!("unix listen failed: {err}"),
        };
        match lis {
            #[cfg(unix)]
            Listener::Unix(_) => {}
            _ => panic!("expected Unix listener"),
        }
        let _ = std::fs::remove_file(path);
    }

    #[tokio::test]
    async fn test_ws_surface_listen() {
        let lis = listen("ws://127.0.0.1:8080/grpc").await.unwrap();
        match lis {
            Listener::Ws(ws) => {
                assert_eq!(ws.host, "127.0.0.1");
                assert_eq!(ws.port, 8080);
                assert_eq!(ws.path, "/grpc");
                assert!(!ws.secure);
            }
            _ => panic!("expected Ws listener"),
        }
    }

    #[tokio::test]
    async fn test_unsupported_uri() {
        let result = listen("ftp://host").await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_rest_sse_uri_is_unsupported() {
        let result = listen("rest+sse://127.0.0.1:8080").await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_tcp_dial_roundtrip() {
        let lis = match listen("tcp://127.0.0.1:0").await {
            Ok(v) => v,
            Err(err) if err.kind() == io::ErrorKind::PermissionDenied => return,
            Err(err) => panic!("tcp listen failed: {err}"),
        };
        let tcp = match lis {
            Listener::Tcp(l) => l,
            _ => panic!("expected Tcp listener"),
        };
        let addr = tcp.local_addr().unwrap();

        let server = tokio::spawn(async move {
            let (mut socket, _) = tcp.accept().await.unwrap();
            let mut buf = [0u8; 4];
            socket.read_exact(&mut buf).await.unwrap();
            socket.write_all(&buf).await.unwrap();
            socket.flush().await.unwrap();
        });

        let mut client = dial_tcp(&format!("tcp://{addr}")).await.unwrap();
        client.write_all(b"ping").await.unwrap();
        client.flush().await.unwrap();

        let mut out = [0u8; 4];
        client.read_exact(&mut out).await.unwrap();
        assert_eq!(&out, b"ping");

        server.await.unwrap();
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn test_unix_dial_roundtrip() {
        let path = std::env::temp_dir().join(format!(
            "holons_rust_unix_{}_{}.sock",
            std::process::id(),
            SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or(Duration::from_secs(0))
                .as_nanos()
        ));
        let uri = format!("unix://{}", path.display());

        let lis = match listen(&uri).await {
            Ok(v) => v,
            Err(err) if err.kind() == io::ErrorKind::PermissionDenied => return,
            Err(err) => panic!("unix listen failed: {err}"),
        };
        let unix = match lis {
            #[cfg(unix)]
            Listener::Unix(l) => l,
            _ => panic!("expected Unix listener"),
        };

        let server = tokio::spawn(async move {
            let (mut socket, _) = unix.accept().await.unwrap();
            let mut buf = [0u8; 3];
            socket.read_exact(&mut buf).await.unwrap();
            socket.write_all(&buf).await.unwrap();
            socket.flush().await.unwrap();
        });

        let mut client = dial_unix(&uri).await.unwrap();
        client.write_all(b"hey").await.unwrap();
        client.flush().await.unwrap();

        let mut out = [0u8; 3];
        client.read_exact(&mut out).await.unwrap();
        assert_eq!(&out, b"hey");

        server.await.unwrap();
        let _ = std::fs::remove_file(path);
    }

    #[tokio::test]
    async fn test_stdio_listen_roundtrip() {
        let (mut client_write, listener_read) = tokio::io::duplex(1024);
        let (listener_write, mut client_read) = tokio::io::duplex(1024);
        let mut stdio = StdioTransport::new(listener_read, listener_write);

        let server = tokio::spawn(async move {
            let mut buf = [0u8; 5];
            stdio.read_exact(&mut buf).await.unwrap();
            stdio.write_all(&buf).await.unwrap();
            stdio.flush().await.unwrap();
        });

        client_write.write_all(b"stdio").await.unwrap();
        client_write.flush().await.unwrap();

        let mut out = [0u8; 5];
        client_read.read_exact(&mut out).await.unwrap();
        assert_eq!(&out, b"stdio");

        server.await.unwrap();
        let _ = listen_stdio().unwrap();
    }
}
