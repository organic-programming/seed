use crate::gen::rust::greeting::v1 as pb;
use holons::gen::holonmeta::v1::{DescribeResponse, FieldDoc, FieldLabel, MethodDoc, ServiceDoc};
use tonic::{Request, Response, Status};

const DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/gen/rust/greeting/v1/greeting_descriptor.bin"
));

#[derive(Default)]
pub(crate) struct GreetingServer;

#[tonic::async_trait]
impl pb::greeting_service_server::GreetingService for GreetingServer {
    async fn list_languages(
        &self,
        request: Request<pb::ListLanguagesRequest>,
    ) -> Result<Response<pb::ListLanguagesResponse>, Status> {
        Ok(Response::new(crate::public::list_languages(
            request.into_inner(),
        )))
    }

    async fn say_hello(
        &self,
        request: Request<pb::SayHelloRequest>,
    ) -> Result<Response<pb::SayHelloResponse>, Status> {
        Ok(Response::new(crate::public::say_hello(
            request.into_inner(),
        )))
    }
}

pub(crate) async fn listen_and_serve(listen_uri: &str, reflect: bool) -> holons::serve::Result<()> {
    holons::serve::run_single_with_options(
        listen_uri,
        pb::greeting_service_server::GreetingServiceServer::new(GreetingServer),
        holons::serve::RunOptions {
            reflect,
            describe_response: Some(describe_response()),
            descriptor_set: Some(DESCRIPTOR_SET.to_vec()),
            ..holons::serve::RunOptions::default()
        },
    )
    .await
}

fn describe_response() -> DescribeResponse {
    DescribeResponse {
        slug: "gabriel-greeting-rust".to_string(),
        motto: "Greets users in 56 languages — a Rust daemon example.".to_string(),
        version: "0.1.19".to_string(),
        services: vec![ServiceDoc {
            name: "greeting.v1.GreetingService".to_string(),
            description: "Language-neutral service contract for the Greeting daemon family. This file carries NO language-specific options and NO manifest data. Each daemon implementation imports it and layers its own metadata on top.".to_string(),
            methods: vec![
                MethodDoc {
                    name: "ListLanguages".to_string(),
                    description: "Returns all available greeting languages.".to_string(),
                    input_type: "greeting.v1.ListLanguagesRequest".to_string(),
                    output_type: "greeting.v1.ListLanguagesResponse".to_string(),
                    input_fields: Vec::new(),
                    output_fields: vec![field_doc(
                        "languages",
                        "greeting.v1.Language",
                        1,
                        "Languages exposed by the daemon.",
                        FieldLabel::Repeated,
                        false,
                        "",
                        vec![
                            field_doc(
                                "code",
                                "string",
                                1,
                                "ISO 639-1 code advertised by the daemon.",
                                FieldLabel::Optional,
                                true,
                                "\"fr\"",
                                Vec::new(),
                            ),
                            field_doc(
                                "name",
                                "string",
                                2,
                                "English display name for the language.",
                                FieldLabel::Optional,
                                true,
                                "\"French\"",
                                Vec::new(),
                            ),
                            field_doc(
                                "native",
                                "string",
                                3,
                                "Native label shown to end users.",
                                FieldLabel::Optional,
                                true,
                                "\"Français\"",
                                Vec::new(),
                            ),
                        ],
                    )],
                    client_streaming: false,
                    server_streaming: false,
                    example_input: "{}".to_string(),
                },
                MethodDoc {
                    name: "SayHello".to_string(),
                    description: "Greets the user in the chosen language.".to_string(),
                    input_type: "greeting.v1.SayHelloRequest".to_string(),
                    output_type: "greeting.v1.SayHelloResponse".to_string(),
                    input_fields: vec![
                        field_doc(
                            "name",
                            "string",
                            1,
                            "Name to greet. If empty, the daemon falls back to a localized default (e.g., \"Mary\", \"Maria\") or \"World\".",
                            FieldLabel::Optional,
                            false,
                            "\"Alice\"",
                            Vec::new(),
                        ),
                        field_doc(
                            "lang_code",
                            "string",
                            2,
                            "ISO 639-1 code chosen by the UI.",
                            FieldLabel::Optional,
                            true,
                            "\"fr\"",
                            Vec::new(),
                        ),
                    ],
                    output_fields: vec![
                        field_doc(
                            "greeting",
                            "string",
                            1,
                            "Localized greeting text returned by the daemon.",
                            FieldLabel::Optional,
                            true,
                            "\"Bonjour, Alice !\"",
                            Vec::new(),
                        ),
                        field_doc(
                            "language",
                            "string",
                            2,
                            "English language name used to resolve the greeting.",
                            FieldLabel::Optional,
                            true,
                            "\"French\"",
                            Vec::new(),
                        ),
                        field_doc(
                            "lang_code",
                            "string",
                            3,
                            "ISO 639-1 code used by the daemon.",
                            FieldLabel::Optional,
                            true,
                            "\"fr\"",
                            Vec::new(),
                        ),
                    ],
                    client_streaming: false,
                    server_streaming: false,
                    example_input: "{\"name\":\"Alice\",\"lang_code\":\"fr\"}".to_string(),
                },
            ],
        }],
    }
}

fn field_doc(
    name: &str,
    field_type: &str,
    number: i32,
    description: &str,
    label: FieldLabel,
    required: bool,
    example: &str,
    nested_fields: Vec<FieldDoc>,
) -> FieldDoc {
    FieldDoc {
        name: name.to_string(),
        r#type: field_type.to_string(),
        number,
        description: description.to_string(),
        label: label as i32,
        map_key_type: String::new(),
        map_value_type: String::new(),
        nested_fields,
        enum_values: Vec::new(),
        required,
        example: example.to_string(),
    }
}
