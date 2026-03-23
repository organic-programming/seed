use serde_json::Value;

fn args(items: &[&str]) -> Vec<String> {
    items.iter().map(|item| (*item).to_string()).collect()
}

fn stdout_string(buffer: Vec<u8>) -> String {
    String::from_utf8(buffer).expect("CLI output should be valid UTF-8")
}

#[test]
fn run_cli_version() {
    let mut stdout = Vec::new();
    let mut stderr = Vec::new();

    let code = crate::cli::run_cli(&args(&["version"]), &mut stdout, &mut stderr);

    assert_eq!(code, 0);
    assert_eq!(stdout_string(stdout).trim(), crate::cli::VERSION);
    assert!(stderr.is_empty());
}

#[test]
fn run_cli_help() {
    let mut stdout = Vec::new();
    let mut stderr = Vec::new();

    let code = crate::cli::run_cli(&args(&["help"]), &mut stdout, &mut stderr);

    assert_eq!(code, 0);
    let output = stdout_string(stdout);
    assert!(output.contains("usage: gabriel-greeting-rust"));
    assert!(output.contains("listLanguages"));
    assert!(stderr.is_empty());
}

#[test]
fn run_cli_list_languages_json() {
    let mut stdout = Vec::new();
    let mut stderr = Vec::new();

    let code = crate::cli::run_cli(
        &args(&["listLanguages", "--format", "json"]),
        &mut stdout,
        &mut stderr,
    );

    assert_eq!(code, 0);
    let payload: Value = serde_json::from_slice(&stdout).expect("valid JSON");
    let languages = payload["languages"]
        .as_array()
        .expect("languages should be an array");
    assert_eq!(languages.len(), 56);
    assert_eq!(languages[0]["code"], "en");
    assert_eq!(languages[0]["name"], "English");
    assert!(stderr.is_empty());
}

#[test]
fn run_cli_say_hello_text() {
    let mut stdout = Vec::new();
    let mut stderr = Vec::new();

    let code = crate::cli::run_cli(
        &args(&["sayHello", "Bob", "fr"]),
        &mut stdout,
        &mut stderr,
    );

    assert_eq!(code, 0);
    assert_eq!(stdout_string(stdout).trim(), "Bonjour Bob");
    assert!(stderr.is_empty());
}

#[test]
fn run_cli_say_hello_defaults_to_english_json() {
    let mut stdout = Vec::new();
    let mut stderr = Vec::new();

    let code = crate::cli::run_cli(&args(&["sayHello", "--json"]), &mut stdout, &mut stderr);

    assert_eq!(code, 0);
    let payload: Value = serde_json::from_slice(&stdout).expect("valid JSON");
    assert_eq!(payload["greeting"], "Hello Mary");
    assert_eq!(payload["language"], "English");
    assert_eq!(payload["langCode"], "en");
    assert!(stderr.is_empty());
}
