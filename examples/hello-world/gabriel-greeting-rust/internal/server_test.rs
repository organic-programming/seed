use crate::gen::rust::greeting::v1 as pb;
use holons::gen::holons::v1::any_value;
use holons::observability::{self, Field};
use std::collections::HashMap;
use tokio::net::TcpListener;
use tokio::sync::oneshot;
use tokio_stream::wrappers::TcpListenerStream;
use tonic::Request;

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

#[tokio::test]
async fn say_hello_emits_otlp_shaped_log_record() {
    observability::reset();
    let mut env = HashMap::new();
    env.insert("OP_OBS".to_string(), "logs,metrics".to_string());
    let obs = observability::configure_from_env(
        observability::Config {
            slug: "gabriel-greeting-rust".to_string(),
            instance_uid: "greeting-test-uid".to_string(),
            ..observability::Config::default()
        },
        &env,
    )
    .unwrap();

    use pb::greeting_service_server::GreetingService;
    let response = crate::internal::server::GreetingServer
        .say_hello(Request::new(pb::SayHelloRequest {
            name: "Ana".to_string(),
            lang_code: "es".to_string(),
        }))
        .await
        .unwrap()
        .into_inner();
    assert_eq!(response.greeting, "Hola Ana");

    let record = obs
        .log_ring
        .as_ref()
        .unwrap()
        .drain()
        .into_iter()
        .find(|entry| entry.message == "Greeted Ana in Spanish (es)")
        .expect("missing greeting log");
    let wire = observability::to_proto_log_record(&record);
    assert_eq!(
        wire.body.as_ref().unwrap().value.as_ref(),
        Some(&any_value::Value::StringValue(
            "Greeted Ana in Spanish (es)".to_string()
        ))
    );
    assert_eq!(
        record.fields.get("transport"),
        Some(&Field::String("unknown".to_string()))
    );
    assert!(matches!(
        record.fields.get("duration_ns"),
        Some(Field::Int64(value)) if *value >= 0
    ));
    assert_eq!(
        observability::string_attribute(&wire.attributes, observability::ATTR_HOLONS_SLUG),
        "gabriel-greeting-rust"
    );
    assert_eq!(
        observability::string_attribute(&wire.attributes, observability::ATTR_SERVICE_NAME),
        "gabriel-greeting-rust"
    );
    assert_eq!(
        observability::string_attribute(&wire.attributes, observability::ATTR_HOLONS_INSTANCE_UID),
        "greeting-test-uid"
    );
    assert_eq!(
        observability::string_attribute(&wire.attributes, observability::ATTR_SERVICE_INSTANCE_ID),
        "greeting-test-uid"
    );

    observability::reset();
}
