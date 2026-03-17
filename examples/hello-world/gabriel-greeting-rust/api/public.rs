use crate::gen::rust::greeting::v1 as pb;
use crate::internal::greetings::{lookup, GREETINGS};

pub fn list_languages(_request: pb::ListLanguagesRequest) -> pb::ListLanguagesResponse {
    let languages = GREETINGS
        .iter()
        .map(|greeting| pb::Language {
            code: greeting.lang_code.to_string(),
            name: greeting.lang_english.to_string(),
            native: greeting.lang_native.to_string(),
        })
        .collect();

    pb::ListLanguagesResponse { languages }
}

pub fn say_hello(request: pb::SayHelloRequest) -> pb::SayHelloResponse {
    let greeting = lookup(&request.lang_code);
    let name = request.name.trim();
    let subject = if name.is_empty() {
        greeting.default_name
    } else {
        name
    };

    pb::SayHelloResponse {
        greeting: render_template(greeting.template, subject),
        language: greeting.lang_english.to_string(),
        lang_code: greeting.lang_code.to_string(),
    }
}

fn render_template(template: &str, name: &str) -> String {
    template.replacen("%s", name, 1)
}
