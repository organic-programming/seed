use std::fmt;

use tonic::{Request, Response, Status};

use crate::greetings::{lookup, GREETINGS};
use crate::proto::greeting_service_server::GreetingService;
use crate::proto::{
    Language, ListLanguagesRequest, ListLanguagesResponse, SayHelloRequest, SayHelloResponse,
};

#[derive(Default)]
pub struct GreetingDaemon;

#[tonic::async_trait]
impl GreetingService for GreetingDaemon {
    async fn list_languages(
        &self,
        _request: Request<ListLanguagesRequest>,
    ) -> Result<Response<ListLanguagesResponse>, Status> {
        let languages = GREETINGS
            .iter()
            .map(|greeting| Language {
                code: greeting.code.to_string(),
                name: greeting.name.to_string(),
                native: greeting.native.to_string(),
            })
            .collect();

        Ok(Response::new(ListLanguagesResponse { languages }))
    }

    async fn say_hello(
        &self,
        request: Request<SayHelloRequest>,
    ) -> Result<Response<SayHelloResponse>, Status> {
        let request = request.into_inner();
        let greeting = lookup(&request.lang_code);
        let name = if request.name.trim().is_empty() {
            "World"
        } else {
            request.name.trim()
        };

        Ok(Response::new(SayHelloResponse {
            greeting: render_template(greeting.template, name),
            language: greeting.name.to_string(),
            lang_code: greeting.code.to_string(),
        }))
    }
}

fn render_template(template: &str, name: &str) -> String {
    if template.contains("%s") {
        template.replacen("%s", name, 1)
    } else {
        format!("{template} {name}")
    }
}

impl fmt::Debug for GreetingDaemon {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("GreetingDaemon")
    }
}
