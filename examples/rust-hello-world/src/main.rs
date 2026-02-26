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
    let addr = "[::]:9090".parse()?;
    eprintln!("gRPC server listening on {}", addr);
    Server::builder()
        .add_service(HelloServiceServer::new(MyHelloService::default()))
        .serve(addr)
        .await?;
    Ok(())
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
}
