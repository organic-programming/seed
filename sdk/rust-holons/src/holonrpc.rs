//! Holon-RPC (JSON-RPC 2.0 over WebSocket and HTTP+SSE) client utilities.

use futures_util::{SinkExt, StreamExt};
use reqwest::header::{ACCEPT, CONTENT_TYPE};
use serde::{Deserialize, Serialize};
use serde_json::{Map, Value};
use std::collections::HashMap;
use std::error::Error;
use std::fmt;
use std::future::Future;
use std::pin::Pin;
use tokio::net::TcpStream;
use tokio_tungstenite::tungstenite::client::IntoClientRequest;
use tokio_tungstenite::tungstenite::http::header::SEC_WEBSOCKET_PROTOCOL;
use tokio_tungstenite::tungstenite::http::HeaderValue;
use tokio_tungstenite::tungstenite::Message;
use tokio_tungstenite::{connect_async, Connector, MaybeTlsStream, WebSocketStream};

const JSON_RPC_VERSION: &str = "2.0";
const CODE_INVALID_REQUEST: i32 = -32600;
const CODE_METHOD_NOT_FOUND: i32 = -32601;
const CODE_INVALID_PARAMS: i32 = -32602;
const CODE_INTERNAL_ERROR: i32 = -32603;

pub type BoxError = Box<dyn Error + Send + Sync>;
pub type Result<T> = std::result::Result<T, BoxError>;
pub type JSONObject = Map<String, Value>;
type HandlerFuture = Pin<Box<dyn Future<Output = Result<JSONObject>> + Send>>;
type Handler = Box<dyn Fn(JSONObject) -> HandlerFuture + Send + Sync>;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TransportMode {
    WebSocket,
    Http,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ResponseError {
    pub code: i32,
    pub message: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub data: Option<Value>,
}

impl fmt::Display for ResponseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        if let Some(data) = &self.data {
            write!(f, "rpc error {}: {} ({data})", self.code, self.message)
        } else {
            write!(f, "rpc error {}: {}", self.code, self.message)
        }
    }
}

impl Error for ResponseError {}

#[derive(Debug, Clone, PartialEq)]
pub struct SSEEvent {
    pub event: String,
    pub id: String,
    pub result: JSONObject,
    pub error: Option<ResponseError>,
}

#[derive(Debug, Serialize, Deserialize)]
struct RpcMessage {
    #[serde(default)]
    jsonrpc: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    id: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    method: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    params: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    error: Option<ResponseError>,
}

pub fn normalize_transport_url(raw: &str) -> Result<(TransportMode, String)> {
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return Err(boxed_err("holon-rpc url is required"));
    }

    if trimmed.starts_with("ws://") || trimmed.starts_with("wss://") {
        return Ok((TransportMode::WebSocket, trimmed.to_string()));
    }

    if trimmed.starts_with("http://") || trimmed.starts_with("https://") {
        return Ok((
            TransportMode::Http,
            trimmed.trim_end_matches('/').to_string(),
        ));
    }

    if let Some(rest) = trimmed.strip_prefix("rest+sse://") {
        return Ok((
            TransportMode::Http,
            format!("http://{}", rest.trim_end_matches('/')),
        ));
    }

    Err(boxed_err(format!(
        "unsupported Holon-RPC transport: {trimmed}"
    )))
}

pub struct Client {
    stream: WebSocketStream<MaybeTlsStream<TcpStream>>,
    handlers: HashMap<String, Handler>,
    next_client_id: u64,
}

impl Client {
    pub async fn connect(url: &str) -> Result<Self> {
        Self::connect_with_connector(url, None).await
    }

    async fn connect_with_connector(url: &str, connector: Option<Connector>) -> Result<Self> {
        let (_, normalized) = normalize_websocket_url(url)?;
        let mut request = normalized.into_client_request()?;
        request.headers_mut().insert(
            SEC_WEBSOCKET_PROTOCOL,
            HeaderValue::from_static("holon-rpc"),
        );

        let (mut stream, response) = if let Some(connector) = connector {
            tokio_tungstenite::connect_async_tls_with_config(request, None, false, Some(connector))
                .await?
        } else {
            connect_async(request).await?
        };
        let protocol = response
            .headers()
            .get(SEC_WEBSOCKET_PROTOCOL)
            .and_then(|value| value.to_str().ok());
        if protocol != Some("holon-rpc") {
            let _ = stream.close(None).await;
            return Err(boxed_err(
                "holon-rpc websocket server did not negotiate holon-rpc",
            ));
        }

        Ok(Self {
            stream,
            handlers: HashMap::new(),
            next_client_id: 0,
        })
    }

    pub fn register<F, Fut>(&mut self, method: impl Into<String>, handler: F)
    where
        F: Fn(JSONObject) -> Fut + Send + Sync + 'static,
        Fut: Future<Output = Result<JSONObject>> + Send + 'static,
    {
        self.handlers.insert(
            method.into(),
            Box::new(move |params| Box::pin(handler(params))),
        );
    }

    pub fn unregister(&mut self, method: &str) {
        self.handlers.remove(method);
    }

    pub async fn invoke(&mut self, method: &str, params: JSONObject) -> Result<JSONObject> {
        let method = method.trim();
        if method.is_empty() {
            return Err(boxed_err("holon-rpc method is required"));
        }

        self.next_client_id += 1;
        let request_id = format!("c{}", self.next_client_id);
        self.write_rpc_message(&RpcMessage {
            jsonrpc: JSON_RPC_VERSION.to_string(),
            id: Some(Value::String(request_id.clone())),
            method: Some(method.to_string()),
            params: Some(Value::Object(params)),
            result: None,
            error: None,
        })
        .await?;

        loop {
            let message = self.next_rpc_message().await?;
            if message.method.is_some() {
                self.handle_request(message).await?;
                continue;
            }

            let Some(response_id) = decode_id(&message.id)? else {
                continue;
            };
            if response_id != request_id {
                continue;
            }

            if message.jsonrpc != JSON_RPC_VERSION {
                return Err(boxed_err("holon-rpc: invalid response"));
            }
            if let Some(error) = message.error {
                return Err(Box::new(error));
            }
            return decode_object(message.result);
        }
    }

    pub async fn close(&mut self) -> Result<()> {
        self.stream.close(None).await?;
        Ok(())
    }

    async fn handle_request(&mut self, message: RpcMessage) -> Result<()> {
        let request_id = message.id;

        if message.jsonrpc != JSON_RPC_VERSION {
            if has_id(&request_id) {
                self.send_error(
                    request_id,
                    ResponseError {
                        code: CODE_INVALID_REQUEST,
                        message: "invalid request".to_string(),
                        data: None,
                    },
                )
                .await?;
            }
            return Ok(());
        }

        let method = message.method.unwrap_or_default().trim().to_string();
        if method.is_empty() {
            if has_id(&request_id) {
                self.send_error(
                    request_id,
                    ResponseError {
                        code: CODE_INVALID_REQUEST,
                        message: "invalid request".to_string(),
                        data: None,
                    },
                )
                .await?;
            }
            return Ok(());
        }

        if method == "rpc.heartbeat" {
            if has_id(&request_id) {
                self.send_result(request_id, JSONObject::new()).await?;
            }
            return Ok(());
        }

        if has_id(&request_id) {
            let Some(request_id_value) = request_id.as_ref() else {
                return Ok(());
            };
            let decoded = decode_string_id(request_id_value).map_err(boxed_err)?;
            if !decoded.starts_with('s') {
                self.send_error(
                    request_id,
                    ResponseError {
                        code: CODE_INVALID_REQUEST,
                        message: "server request id must start with 's'".to_string(),
                        data: None,
                    },
                )
                .await?;
                return Ok(());
            }
        }

        let params = match decode_params(message.params) {
            Ok(params) => params,
            Err(err) => {
                if has_id(&request_id) {
                    self.send_error(
                        request_id,
                        ResponseError {
                            code: CODE_INVALID_PARAMS,
                            message: err,
                            data: None,
                        },
                    )
                    .await?;
                }
                return Ok(());
            }
        };

        let Some(handler) = self.handlers.get(&method) else {
            if has_id(&request_id) {
                self.send_error(
                    request_id,
                    ResponseError {
                        code: CODE_METHOD_NOT_FOUND,
                        message: format!("method {method:?} not found"),
                        data: None,
                    },
                )
                .await?;
            }
            return Ok(());
        };

        let result = handler(params).await;
        if !has_id(&request_id) {
            return Ok(());
        }

        match result {
            Ok(result) => self.send_result(request_id, result).await?,
            Err(err) => {
                if let Some(response) = err.downcast_ref::<ResponseError>() {
                    self.send_error(request_id, response.clone()).await?;
                } else {
                    self.send_error(
                        request_id,
                        ResponseError {
                            code: CODE_INTERNAL_ERROR,
                            message: "internal error".to_string(),
                            data: None,
                        },
                    )
                    .await?;
                }
            }
        }

        Ok(())
    }

    async fn send_result(&mut self, id: Option<Value>, result: JSONObject) -> Result<()> {
        self.write_rpc_message(&RpcMessage {
            jsonrpc: JSON_RPC_VERSION.to_string(),
            id,
            method: None,
            params: None,
            result: Some(Value::Object(result)),
            error: None,
        })
        .await
    }

    async fn send_error(&mut self, id: Option<Value>, error: ResponseError) -> Result<()> {
        self.write_rpc_message(&RpcMessage {
            jsonrpc: JSON_RPC_VERSION.to_string(),
            id,
            method: None,
            params: None,
            result: None,
            error: Some(error),
        })
        .await
    }

    async fn write_rpc_message(&mut self, message: &RpcMessage) -> Result<()> {
        let payload = serde_json::to_string(message)?;
        self.stream.send(Message::Text(payload)).await?;
        Ok(())
    }

    async fn next_rpc_message(&mut self) -> Result<RpcMessage> {
        loop {
            let Some(next) = self.stream.next().await else {
                return Err(boxed_err("holon-rpc connection closed"));
            };

            match next? {
                Message::Text(text) => return Ok(serde_json::from_str(&text)?),
                Message::Binary(binary) => return Ok(serde_json::from_slice(&binary)?),
                Message::Ping(payload) => {
                    self.stream.send(Message::Pong(payload)).await?;
                }
                Message::Pong(_) => {}
                Message::Close(_) => return Err(boxed_err("holon-rpc connection closed")),
                _ => {}
            }
        }
    }
}

pub struct HTTPClient {
    base_url: String,
    client: reqwest::Client,
}

impl HTTPClient {
    pub fn new(base_url: &str) -> Result<Self> {
        Self::with_client(base_url, reqwest::Client::new())
    }

    pub fn with_client(base_url: &str, client: reqwest::Client) -> Result<Self> {
        let (_, normalized) = normalize_http_url(base_url)?;
        Ok(Self {
            base_url: normalized,
            client,
        })
    }

    pub async fn invoke(&self, method: &str, params: JSONObject) -> Result<JSONObject> {
        let response = self
            .client
            .post(self.method_url(method)?)
            .header(CONTENT_TYPE, "application/json")
            .header(ACCEPT, "application/json")
            .json(&Value::Object(params))
            .send()
            .await?;

        decode_http_response(response).await
    }

    pub async fn stream(&self, method: &str, params: JSONObject) -> Result<Vec<SSEEvent>> {
        let response = self
            .client
            .post(self.method_url(method)?)
            .header(CONTENT_TYPE, "application/json")
            .header(ACCEPT, "text/event-stream")
            .json(&Value::Object(params))
            .send()
            .await?;

        read_sse_events(response).await
    }

    pub async fn stream_query(
        &self,
        method: &str,
        params: &HashMap<String, String>,
    ) -> Result<Vec<SSEEvent>> {
        let response = self
            .client
            .get(self.method_url(method)?)
            .query(params)
            .header(ACCEPT, "text/event-stream")
            .send()
            .await?;

        read_sse_events(response).await
    }

    fn method_url(&self, method: &str) -> Result<String> {
        let method = method.trim().trim_matches('/');
        if method.is_empty() {
            return Err(boxed_err("holon-rpc method is required"));
        }
        Ok(format!("{}/{}", self.base_url, method))
    }
}

fn normalize_websocket_url(raw: &str) -> Result<(TransportMode, String)> {
    let (mode, normalized) = normalize_transport_url(raw)?;
    if mode != TransportMode::WebSocket {
        return Err(boxed_err(format!(
            "expected ws:// or wss:// url, got {raw:?}"
        )));
    }
    Ok((mode, normalized))
}

fn normalize_http_url(raw: &str) -> Result<(TransportMode, String)> {
    let (mode, normalized) = normalize_transport_url(raw)?;
    if mode != TransportMode::Http {
        return Err(boxed_err(format!(
            "expected http://, https://, or rest+sse:// url, got {raw:?}"
        )));
    }
    Ok((mode, normalized))
}

async fn decode_http_response(response: reqwest::Response) -> Result<JSONObject> {
    let status = response.status();
    let body = response.bytes().await?;

    if let Ok(message) = serde_json::from_slice::<RpcMessage>(&body) {
        if let Some(error) = message.error {
            return Err(Box::new(error));
        }
        if message.jsonrpc == JSON_RPC_VERSION || message.result.is_some() {
            return decode_object(message.result);
        }
    }

    if status.is_client_error() || status.is_server_error() {
        return Err(boxed_err(format!(
            "holon-rpc http status {}",
            status.as_u16()
        )));
    }

    let value = serde_json::from_slice::<Value>(&body)?;
    decode_object(Some(value))
}

async fn read_sse_events(response: reqwest::Response) -> Result<Vec<SSEEvent>> {
    let status = response.status();
    let body = response.text().await?;

    if status.is_client_error() || status.is_server_error() {
        return decode_http_error_text(status.as_u16(), &body);
    }

    let mut events = Vec::new();
    let mut event = String::new();
    let mut id = String::new();
    let mut data_lines: Vec<String> = Vec::new();

    let flush = |events: &mut Vec<SSEEvent>,
                 event: &mut String,
                 id: &mut String,
                 data_lines: &mut Vec<String>|
     -> Result<()> {
        if event.is_empty() && id.is_empty() && data_lines.is_empty() {
            return Ok(());
        }

        let name = if event.is_empty() {
            "message".to_string()
        } else {
            event.clone()
        };
        let payload = data_lines.join("\n");
        let mut parsed = SSEEvent {
            event: name.clone(),
            id: id.clone(),
            result: JSONObject::new(),
            error: None,
        };

        match name.as_str() {
            "message" | "error" if !payload.trim().is_empty() => {
                let message: RpcMessage = serde_json::from_str(&payload)?;
                if let Some(error) = message.error {
                    parsed.error = Some(error);
                } else {
                    parsed.result = decode_object(message.result)?;
                }
            }
            "done" => {}
            _ => {}
        }

        events.push(parsed);
        event.clear();
        id.clear();
        data_lines.clear();
        Ok(())
    };

    for line in body.lines() {
        if line.is_empty() {
            flush(&mut events, &mut event, &mut id, &mut data_lines)?;
            continue;
        }

        if let Some(value) = line.strip_prefix("event:") {
            event = value.trim().to_string();
            continue;
        }
        if let Some(value) = line.strip_prefix("id:") {
            id = value.trim().to_string();
            continue;
        }
        if let Some(value) = line.strip_prefix("data:") {
            data_lines.push(value.trim().to_string());
        }
    }

    flush(&mut events, &mut event, &mut id, &mut data_lines)?;
    Ok(events)
}

fn decode_http_error_text(status: u16, body: &str) -> Result<Vec<SSEEvent>> {
    if let Ok(message) = serde_json::from_str::<RpcMessage>(body) {
        if let Some(error) = message.error {
            return Err(Box::new(error));
        }
    }
    Err(boxed_err(format!("holon-rpc http status {status}")))
}

fn has_id(id: &Option<Value>) -> bool {
    !matches!(id, None | Some(Value::Null))
}

fn decode_id(id: &Option<Value>) -> Result<Option<String>> {
    let Some(id) = id else {
        return Ok(None);
    };
    if id.is_null() {
        return Ok(None);
    }
    Ok(Some(decode_string_id(id).map_err(boxed_err)?))
}

fn decode_string_id(id: &Value) -> std::result::Result<String, String> {
    match id {
        Value::String(value) => Ok(value.clone()),
        _ => Err("id must be a string".to_string()),
    }
}

fn decode_params(value: Option<Value>) -> std::result::Result<JSONObject, String> {
    match value {
        None | Some(Value::Null) => Ok(JSONObject::new()),
        Some(Value::Object(params)) => Ok(params),
        _ => Err("params must be an object".to_string()),
    }
}

fn decode_object(value: Option<Value>) -> Result<JSONObject> {
    match value {
        None | Some(Value::Null) => Ok(JSONObject::new()),
        Some(Value::Object(object)) => Ok(object),
        Some(other) => {
            let mut wrapped = JSONObject::new();
            wrapped.insert("value".to_string(), other);
            Ok(wrapped)
        }
    }
}

fn boxed_err(message: impl Into<String>) -> BoxError {
    Box::new(std::io::Error::new(
        std::io::ErrorKind::Other,
        message.into(),
    ))
}

#[cfg(test)]
mod tests {
    use super::*;
    use rcgen::generate_simple_self_signed;
    use serde_json::json;
    use std::io;
    use std::sync::Arc;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::TcpListener;
    use tokio_rustls::rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
    use tokio_rustls::rustls::{ClientConfig, RootCertStore, ServerConfig};
    use tokio_rustls::TlsAcceptor;
    use tokio_tungstenite::accept_hdr_async;
    use tokio_tungstenite::tungstenite::handshake::server::{Request, Response};

    #[test]
    fn test_normalize_transport_url_supports_rest_sse_alias() {
        let (mode, normalized) =
            normalize_transport_url("rest+sse://127.0.0.1:8080/api/v1/rpc").unwrap();
        assert_eq!(mode, TransportMode::Http);
        assert_eq!(normalized, "http://127.0.0.1:8080/api/v1/rpc");
    }

    #[tokio::test]
    async fn test_client_invokes_over_ws() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let done = spawn_websocket_echo_server(listener, false);

        let mut client = Client::connect(&format!("ws://{address}/rpc"))
            .await
            .unwrap();
        let mut params = JSONObject::new();
        params.insert("message".to_string(), Value::String("hola".to_string()));
        let response = client.invoke("echo.v1.Echo/Ping", params).await.unwrap();
        assert_eq!(
            response.get("message"),
            Some(&Value::String("hola".to_string()))
        );
        client.close().await.unwrap();

        done.await.unwrap();
    }

    #[tokio::test]
    async fn test_client_handles_server_callback_during_invoke() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let done = spawn_websocket_callback_server(listener);

        let mut client = Client::connect(&format!("ws://{address}/rpc"))
            .await
            .unwrap();
        client.register("client.v1.Client/Hello", |params| async move {
            let name = params
                .get("name")
                .and_then(Value::as_str)
                .unwrap_or("unknown");
            let mut result = JSONObject::new();
            result.insert("message".to_string(), Value::String(format!("pong:{name}")));
            Ok(result)
        });

        let response = client
            .invoke("echo.v1.Echo/CallClient", JSONObject::new())
            .await
            .unwrap();
        assert_eq!(
            response.get("message"),
            Some(&Value::String("pong:from-server".to_string()))
        );

        client.close().await.unwrap();
        done.await.unwrap();
    }

    #[tokio::test]
    async fn test_client_invokes_over_wss() {
        install_rustls_provider();

        let cert = generate_simple_self_signed(vec!["localhost".to_string()]).unwrap();
        let cert_der = CertificateDer::from(cert.cert.der().to_vec());
        let key_der = PrivateKeyDer::from(PrivatePkcs8KeyDer::from(cert.key_pair.serialize_der()));
        let server_config = ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(vec![cert_der.clone()], key_der)
            .unwrap();
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let tls_acceptor = TlsAcceptor::from(Arc::new(server_config));
        let done = spawn_tls_websocket_echo_server(listener, tls_acceptor);

        let mut roots = RootCertStore::empty();
        roots.add(cert_der).unwrap();
        let client_config = ClientConfig::builder()
            .with_root_certificates(roots)
            .with_no_client_auth();

        let mut client = Client::connect_with_connector(
            &format!("wss://localhost:{}/rpc", address.port()),
            Some(Connector::Rustls(Arc::new(client_config))),
        )
        .await
        .unwrap();

        let mut params = JSONObject::new();
        params.insert("message".to_string(), Value::String("secure".to_string()));
        let response = client.invoke("echo.v1.Echo/Ping", params).await.unwrap();
        assert_eq!(
            response.get("message"),
            Some(&Value::String("secure".to_string()))
        );
        client.close().await.unwrap();

        done.await.unwrap();
    }

    #[tokio::test]
    async fn test_http_client_invokes_over_rest_sse() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let done = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let request = read_http_request(&mut stream).await.unwrap();
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/api/v1/rpc/echo.v1.Echo/Ping");
            assert_eq!(
                request.headers.get("accept").map(String::as_str),
                Some("application/json")
            );
            let body = String::from_utf8(request.body).unwrap();
            assert_eq!(body, r#"{"message":"hello"}"#);

            let response = serde_json::to_vec(&json!({
                "jsonrpc": "2.0",
                "id": "h1",
                "result": { "message": "hello" }
            }))
            .unwrap();
            write_http_response(&mut stream, "200 OK", "application/json", &response)
                .await
                .unwrap();
        });

        let client = HTTPClient::new(&format!("rest+sse://{address}/api/v1/rpc")).unwrap();
        let mut params = JSONObject::new();
        params.insert("message".to_string(), Value::String("hello".to_string()));
        let response = client.invoke("echo.v1.Echo/Ping", params).await.unwrap();
        assert_eq!(
            response.get("message"),
            Some(&Value::String("hello".to_string()))
        );

        done.await.unwrap();
    }

    #[tokio::test]
    async fn test_http_client_streams_sse_post() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let done = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let request = read_http_request(&mut stream).await.unwrap();
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/api/v1/rpc/build.v1.Build/Watch");
            assert_eq!(
                request.headers.get("accept").map(String::as_str),
                Some("text/event-stream")
            );
            let body = String::from_utf8(request.body).unwrap();
            assert_eq!(body, r#"{"project":"myapp"}"#);

            let payload = concat!(
                "event: message\r\n",
                "id: 1\r\n",
                "data: {\"jsonrpc\":\"2.0\",\"id\":\"h1\",\"result\":{\"status\":\"building\"}}\r\n",
                "\r\n",
                "event: message\r\n",
                "id: 2\r\n",
                "data: {\"jsonrpc\":\"2.0\",\"id\":\"h1\",\"result\":{\"status\":\"done\"}}\r\n",
                "\r\n",
                "event: done\r\n",
                "data:\r\n",
                "\r\n"
            );
            write_http_stream_response(&mut stream, payload)
                .await
                .unwrap();
        });

        let client = HTTPClient::new(&format!("http://{address}/api/v1/rpc")).unwrap();
        let mut params = JSONObject::new();
        params.insert("project".to_string(), Value::String("myapp".to_string()));
        let events = client.stream("build.v1.Build/Watch", params).await.unwrap();
        assert_eq!(events.len(), 3);
        assert_eq!(events[0].event, "message");
        assert_eq!(events[0].id, "1");
        assert_eq!(
            events[0].result.get("status"),
            Some(&Value::String("building".to_string()))
        );
        assert_eq!(
            events[1].result.get("status"),
            Some(&Value::String("done".to_string()))
        );
        assert_eq!(events[2].event, "done");

        done.await.unwrap();
    }

    #[tokio::test]
    async fn test_http_client_streams_sse_query() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let done = tokio::spawn(async move {
            let (mut stream, _) = listener.accept().await.unwrap();
            let request = read_http_request(&mut stream).await.unwrap();
            assert_eq!(request.method, "GET");
            assert_eq!(
                request.path,
                "/api/v1/rpc/build.v1.Build/Watch?project=myapp"
            );
            let payload = concat!(
                "event: message\r\n",
                "id: 1\r\n",
                "data: {\"jsonrpc\":\"2.0\",\"id\":\"h1\",\"result\":{\"status\":\"watching\"}}\r\n",
                "\r\n",
                "event: done\r\n",
                "data:\r\n",
                "\r\n"
            );
            write_http_stream_response(&mut stream, payload)
                .await
                .unwrap();
        });

        let client = HTTPClient::new(&format!("http://{address}/api/v1/rpc")).unwrap();
        let params = HashMap::from([("project".to_string(), "myapp".to_string())]);
        let events = client
            .stream_query("build.v1.Build/Watch", &params)
            .await
            .unwrap();
        assert_eq!(events.len(), 2);
        assert_eq!(
            events[0].result.get("status"),
            Some(&Value::String("watching".to_string()))
        );
        assert_eq!(events[1].event, "done");

        done.await.unwrap();
    }

    fn spawn_websocket_echo_server(
        listener: TcpListener,
        secure: bool,
    ) -> tokio::task::JoinHandle<()> {
        tokio::spawn(async move {
            let (stream, _) = listener.accept().await.unwrap();
            let mut websocket = accept_test_websocket(stream).await.unwrap();
            let message = read_text_message(&mut websocket).await.unwrap();
            let request: RpcMessage = serde_json::from_str(&message).unwrap();
            assert_eq!(request.method.as_deref(), Some("echo.v1.Echo/Ping"));
            let response = RpcMessage {
                jsonrpc: JSON_RPC_VERSION.to_string(),
                id: request.id,
                method: None,
                params: None,
                result: request.params,
                error: None,
            };
            websocket
                .send(Message::Text(serde_json::to_string(&response).unwrap()))
                .await
                .unwrap();
            let _ = websocket.close(None).await;
            let _ = secure;
        })
    }

    fn spawn_websocket_callback_server(listener: TcpListener) -> tokio::task::JoinHandle<()> {
        tokio::spawn(async move {
            let (stream, _) = listener.accept().await.unwrap();
            let mut websocket = accept_test_websocket(stream).await.unwrap();
            let message = read_text_message(&mut websocket).await.unwrap();
            let request: RpcMessage = serde_json::from_str(&message).unwrap();
            assert_eq!(request.method.as_deref(), Some("echo.v1.Echo/CallClient"));

            let callback = RpcMessage {
                jsonrpc: JSON_RPC_VERSION.to_string(),
                id: Some(Value::String("s1".to_string())),
                method: Some("client.v1.Client/Hello".to_string()),
                params: Some(json!({ "name": "from-server" })),
                result: None,
                error: None,
            };
            websocket
                .send(Message::Text(serde_json::to_string(&callback).unwrap()))
                .await
                .unwrap();

            let callback_response: RpcMessage =
                serde_json::from_str(&read_text_message(&mut websocket).await.unwrap()).unwrap();
            let response = RpcMessage {
                jsonrpc: JSON_RPC_VERSION.to_string(),
                id: request.id,
                method: None,
                params: None,
                result: callback_response.result,
                error: None,
            };
            websocket
                .send(Message::Text(serde_json::to_string(&response).unwrap()))
                .await
                .unwrap();
            let _ = websocket.close(None).await;
        })
    }

    fn spawn_tls_websocket_echo_server(
        listener: TcpListener,
        tls_acceptor: TlsAcceptor,
    ) -> tokio::task::JoinHandle<()> {
        tokio::spawn(async move {
            let (stream, _) = listener.accept().await.unwrap();
            let stream = tls_acceptor.accept(stream).await.unwrap();
            let mut websocket = accept_test_websocket(stream).await.unwrap();
            let message = read_text_message(&mut websocket).await.unwrap();
            let request: RpcMessage = serde_json::from_str(&message).unwrap();
            let response = RpcMessage {
                jsonrpc: JSON_RPC_VERSION.to_string(),
                id: request.id,
                method: None,
                params: None,
                result: request.params,
                error: None,
            };
            websocket
                .send(Message::Text(serde_json::to_string(&response).unwrap()))
                .await
                .unwrap();
            let _ = websocket.close(None).await;
        })
    }

    async fn accept_test_websocket<S>(stream: S) -> io::Result<WebSocketStream<S>>
    where
        S: tokio::io::AsyncRead + tokio::io::AsyncWrite + Unpin,
    {
        accept_hdr_async(stream, |request: &Request, mut response: Response| {
            let protocol = request
                .headers()
                .get(SEC_WEBSOCKET_PROTOCOL)
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default();
            if protocol != "holon-rpc" {
                return Err(
                    tokio_tungstenite::tungstenite::handshake::server::ErrorResponse::new(Some(
                        "missing holon-rpc subprotocol".to_string(),
                    )),
                );
            }
            response.headers_mut().insert(
                SEC_WEBSOCKET_PROTOCOL,
                HeaderValue::from_static("holon-rpc"),
            );
            Ok(response)
        })
        .await
        .map_err(io::Error::other)
    }

    async fn read_text_message<S>(websocket: &mut WebSocketStream<S>) -> io::Result<String>
    where
        S: tokio::io::AsyncRead + tokio::io::AsyncWrite + Unpin,
    {
        loop {
            let Some(next) = websocket.next().await else {
                return Err(io::Error::other("websocket closed"));
            };
            match next.map_err(io::Error::other)? {
                Message::Text(text) => return Ok(text),
                Message::Ping(payload) => websocket
                    .send(Message::Pong(payload))
                    .await
                    .map_err(io::Error::other)?,
                Message::Pong(_) => {}
                Message::Close(_) => return Err(io::Error::other("websocket closed")),
                other => return Err(io::Error::other(format!("unexpected message: {other:?}"))),
            }
        }
    }

    struct HTTPRequest {
        method: String,
        path: String,
        headers: HashMap<String, String>,
        body: Vec<u8>,
    }

    async fn read_http_request(stream: &mut TcpStream) -> io::Result<HTTPRequest> {
        let mut buffer = Vec::new();
        let mut chunk = [0u8; 1024];
        let headers_end = loop {
            let read = stream.read(&mut chunk).await?;
            if read == 0 {
                return Err(io::Error::new(
                    io::ErrorKind::UnexpectedEof,
                    "unexpected eof before headers",
                ));
            }
            buffer.extend_from_slice(&chunk[..read]);
            if let Some(index) = find_bytes(&buffer, b"\r\n\r\n") {
                break index;
            }
        };

        let header_bytes = &buffer[..headers_end];
        let header_text = String::from_utf8_lossy(header_bytes);
        let mut lines = header_text.lines();
        let request_line = lines
            .next()
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidData, "missing request line"))?;
        let mut parts = request_line.split_whitespace();
        let method = parts
            .next()
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidData, "missing method"))?
            .to_string();
        let path = parts
            .next()
            .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidData, "missing path"))?
            .to_string();

        let mut headers = HashMap::new();
        for line in lines {
            if let Some((name, value)) = line.split_once(':') {
                headers.insert(name.trim().to_lowercase(), value.trim().to_string());
            }
        }

        let content_length = headers
            .get("content-length")
            .and_then(|value| value.parse::<usize>().ok())
            .unwrap_or(0);
        let body_start = headers_end + 4;
        let mut body = buffer[body_start..].to_vec();
        while body.len() < content_length {
            let read = stream.read(&mut chunk).await?;
            if read == 0 {
                return Err(io::Error::new(
                    io::ErrorKind::UnexpectedEof,
                    "unexpected eof before body",
                ));
            }
            body.extend_from_slice(&chunk[..read]);
        }
        body.truncate(content_length);

        Ok(HTTPRequest {
            method,
            path,
            headers,
            body,
        })
    }

    async fn write_http_response(
        stream: &mut TcpStream,
        status: &str,
        content_type: &str,
        body: &[u8],
    ) -> io::Result<()> {
        let headers = format!(
            "HTTP/1.1 {status}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            body.len()
        );
        stream.write_all(headers.as_bytes()).await?;
        stream.write_all(body).await?;
        stream.shutdown().await
    }

    async fn write_http_stream_response(stream: &mut TcpStream, body: &str) -> io::Result<()> {
        let headers = concat!(
            "HTTP/1.1 200 OK\r\n",
            "Content-Type: text/event-stream\r\n",
            "Cache-Control: no-cache\r\n",
            "Connection: close\r\n",
            "\r\n"
        );
        stream.write_all(headers.as_bytes()).await?;
        stream.write_all(body.as_bytes()).await?;
        stream.shutdown().await
    }

    fn find_bytes(haystack: &[u8], needle: &[u8]) -> Option<usize> {
        haystack
            .windows(needle.len())
            .position(|window| window == needle)
    }

    fn install_rustls_provider() {
        static INIT: std::sync::Once = std::sync::Once::new();
        INIT.call_once(|| {
            let _ = tokio_rustls::rustls::crypto::ring::default_provider().install_default();
        });
    }
}
