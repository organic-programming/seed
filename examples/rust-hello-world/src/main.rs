use std::net::SocketAddr;
use tonic::{transport::Server, Request, Response, Status};

pub mod hello {
    tonic::include_proto!("hello.v1");
}

use hello::hello_service_server::{HelloService, HelloServiceServer};
use hello::{GreetRequest, GreetResponse};

/// HelloService implementation — pure deterministic.
#[derive(Debug, Default)]
pub struct MyHelloService;

#[tonic::async_trait]
impl HelloService for MyHelloService {
    async fn greet(&self, req: Request<GreetRequest>) -> Result<Response<GreetResponse>, Status> {
        let name = if req.get_ref().name.is_empty() {
            "World".to_string()
        } else {
            req.get_ref().name.clone()
        };
        Ok(Response::new(GreetResponse {
            message: format!("Hello, {}!", name),
        }))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let args: Vec<String> = std::env::args().skip(1).collect();
    let serve_args: &[String] = if args.first().map(String::as_str) == Some("serve") {
        &args[1..]
    } else {
        &args
    };
    let listen_uri = holons::serve::parse_flags(serve_args);
    let addr = parse_tcp_addr(&listen_uri)?;
    eprintln!("gRPC server listening on {}", addr);
    Server::builder()
        .add_service(HelloServiceServer::new(MyHelloService::default()))
        .serve(addr)
        .await?;
    Ok(())
}

fn parse_tcp_addr(listen_uri: &str) -> Result<SocketAddr, Box<dyn std::error::Error>> {
    let raw = listen_uri
        .strip_prefix("tcp://")
        .ok_or_else(|| format!("unsupported listen URI {listen_uri:?}"))?;
    let normalized = if let Some(rest) = raw.strip_prefix(':') {
        format!("0.0.0.0:{rest}")
    } else {
        raw.to_string()
    };
    Ok(normalized.parse()?)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_greet_with_name() {
        let svc = MyHelloService;
        let req = Request::new(GreetRequest {
            name: "Alice".into(),
        });
        let resp = svc.greet(req).await.unwrap();
        assert_eq!(resp.get_ref().message, "Hello, Alice!");
    }

    #[tokio::test]
    async fn test_greet_default() {
        let svc = MyHelloService;
        let req = Request::new(GreetRequest { name: "".into() });
        let resp = svc.greet(req).await.unwrap();
        assert_eq!(resp.get_ref().message, "Hello, World!");
    }

    #[test]
    fn test_parse_tcp_addr() {
        assert_eq!(
            parse_tcp_addr("tcp://:9090").unwrap().to_string(),
            "0.0.0.0:9090"
        );
        assert_eq!(
            parse_tcp_addr("tcp://127.0.0.1:8080").unwrap().to_string(),
            "127.0.0.1:8080"
        );
    }
}
