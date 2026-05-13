fn main() {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let mut stdout = std::io::stdout();
    let mut stderr = std::io::stderr();
    let code = cascade_node_rust::cli::run_cli(&args, &mut stdout, &mut stderr);
    std::process::exit(code);
}
