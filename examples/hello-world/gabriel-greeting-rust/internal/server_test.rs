use crate::gen::rust::greeting::v1 as pb;
use tokio::net::TcpListener;
use tokio::sync::oneshot;
use tokio_stream::wrappers::TcpListenerStream;

struct TestServer {
    client: pb::greeting_service_client::GreetingServiceClient<tonic::transport::Channel>,
    shutdown: Option<oneshot::Sender<()>>,
}

impl Drop for TestServer {
    fn drop(&mut self) {
        if let Some(shutdown) = self.shutdown.take() {
            let _ = shutdown.send(());
        }
    }
}

async fn start_server() -> TestServer {
    let listener = TcpListener::bind("127.0.0.1:0")
        .await
        .expect("should bind an ephemeral test port");
    let address = listener
        .local_addr()
        .expect("listener should have an address");
    let incoming = TcpListenerStream::new(listener);
    let (shutdown_tx, shutdown_rx) = oneshot::channel();

    tokio::spawn(async move {
        let result = tonic::transport::Server::builder()
            .add_service(pb::greeting_service_server::GreetingServiceServer::new(
                crate::internal::server::GreetingServer,
            ))
            .serve_with_incoming_shutdown(incoming, async {
                let _ = shutdown_rx.await;
            })
            .await;

        if let Err(error) = result {
            panic!("test server failed: {error}");
        }
    });

    let client =
        pb::greeting_service_client::GreetingServiceClient::connect(format!("http://{address}"))
            .await
            .expect("client should connect to the test server");

    TestServer {
        client,
        shutdown: Some(shutdown_tx),
    }
}

#[tokio::test]
async fn list_languages_returns_all_languages() {
    let mut server = start_server().await;

    let response = server
        .client
        .list_languages(pb::ListLanguagesRequest::default())
        .await
        .expect("ListLanguages should succeed")
        .into_inner();

    assert_eq!(response.languages.len(), 56);
}

#[tokio::test]
async fn list_languages_populates_required_fields() {
    let mut server = start_server().await;

    let response = server
        .client
        .list_languages(pb::ListLanguagesRequest::default())
        .await
        .expect("ListLanguages should succeed")
        .into_inner();

    for language in response.languages {
        assert!(!language.code.is_empty());
        assert!(!language.name.is_empty());
        assert!(!language.native.is_empty());
    }
}

#[tokio::test]
async fn say_hello_uses_requested_language() {
    let mut server = start_server().await;

    let response = server
        .client
        .say_hello(pb::SayHelloRequest {
            name: "Bob".to_string(),
            lang_code: "fr".to_string(),
        })
        .await
        .expect("SayHello should succeed")
        .into_inner();

    assert_eq!(response.greeting, "Bonjour Bob");
    assert_eq!(response.language, "French");
    assert_eq!(response.lang_code, "fr");
}

#[tokio::test]
async fn say_hello_uses_localized_default_name() {
    let mut server = start_server().await;

    let response = server
        .client
        .say_hello(pb::SayHelloRequest {
            name: String::new(),
            lang_code: "fr".to_string(),
        })
        .await
        .expect("SayHello should succeed")
        .into_inner();

    assert_eq!(response.greeting, "Bonjour Marie");
    assert_eq!(response.lang_code, "fr");
}

#[tokio::test]
async fn say_hello_falls_back_to_english() {
    let mut server = start_server().await;

    let response = server
        .client
        .say_hello(pb::SayHelloRequest {
            name: "Bob".to_string(),
            lang_code: "xx".to_string(),
        })
        .await
        .expect("SayHello should succeed")
        .into_inner();

    assert_eq!(response.greeting, "Hello Bob");
    assert_eq!(response.lang_code, "en");
}
