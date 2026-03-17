use crate::gen::rust::greeting::v1 as pb;

#[test]
fn list_languages_includes_english() {
    let response = crate::public::list_languages(pb::ListLanguagesRequest::default());

    assert!(!response.languages.is_empty());
    let english = response
        .languages
        .iter()
        .find(|language| language.code == "en")
        .expect("English should be present");

    assert_eq!(english.name, "English");
    assert_eq!(english.native, "English");
}

#[test]
fn say_hello_uses_requested_language() {
    let response = crate::public::say_hello(pb::SayHelloRequest {
        name: "Alice".to_string(),
        lang_code: "fr".to_string(),
    });

    assert_eq!(response.greeting, "Bonjour Alice");
    assert_eq!(response.language, "French");
    assert_eq!(response.lang_code, "fr");
}

#[test]
fn say_hello_uses_localized_default_name() {
    let response = crate::public::say_hello(pb::SayHelloRequest {
        name: String::new(),
        lang_code: "ja".to_string(),
    });

    assert_eq!(response.greeting, "こんにちは、マリアさん");
    assert_eq!(response.language, "Japanese");
    assert_eq!(response.lang_code, "ja");
}

#[test]
fn say_hello_falls_back_to_english_for_unknown_language() {
    let response = crate::public::say_hello(pb::SayHelloRequest {
        name: String::new(),
        lang_code: "unknown".to_string(),
    });

    assert_eq!(response.greeting, "Hello Mary");
    assert_eq!(response.language, "English");
    assert_eq!(response.lang_code, "en");
}
