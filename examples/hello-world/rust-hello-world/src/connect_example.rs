use bytes::{Buf, BufMut};
use std::env;
use std::error::Error;
use std::fs;
use std::path::PathBuf;
use tempfile::TempDir;
use tonic::client::Grpc;
use tonic::codec::{Codec, DecodeBuf, Decoder, EncodeBuf, Encoder};
use tonic::codegen::http::uri::PathAndQuery;
use tonic::{Request, Status};

#[derive(Clone, Default)]
struct StringCodec;

#[derive(Clone, Default)]
struct StringEncoder;

#[derive(Clone, Default)]
struct StringDecoder;

impl Codec for StringCodec {
    type Encode = String;
    type Decode = String;
    type Encoder = StringEncoder;
    type Decoder = StringDecoder;

    fn encoder(&mut self) -> Self::Encoder {
        StringEncoder
    }

    fn decoder(&mut self) -> Self::Decoder {
        StringDecoder
    }
}

impl Encoder for StringEncoder {
    type Item = String;
    type Error = Status;

    fn encode(&mut self, item: Self::Item, dst: &mut EncodeBuf<'_>) -> Result<(), Self::Error> {
        dst.reserve(item.len());
        dst.put_slice(item.as_bytes());
        Ok(())
    }
}

impl Decoder for StringDecoder {
    type Item = String;
    type Error = Status;

    fn decode(&mut self, src: &mut DecodeBuf<'_>) -> Result<Option<Self::Item>, Self::Error> {
        let bytes = src.copy_to_bytes(src.remaining());
        String::from_utf8(bytes.to_vec())
            .map(Some)
            .map_err(|err| Status::internal(err.to_string()))
    }
}

struct CurrentDirGuard {
    previous: PathBuf,
}

impl CurrentDirGuard {
    fn enter(path: &std::path::Path) -> Result<Self, Box<dyn Error>> {
        let previous = env::current_dir()?;
        env::set_current_dir(path)?;
        Ok(Self { previous })
    }
}

impl Drop for CurrentDirGuard {
    fn drop(&mut self) {
        let _ = env::set_current_dir(&self.previous);
    }
}

fn sdk_echo_server() -> Result<PathBuf, Box<dyn Error>> {
    let path = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../../sdk/rust-holons/bin/echo-server")
        .canonicalize()?;
    if !path.is_file() {
        return Err(format!("echo-server not found at {}", path.display()).into());
    }
    Ok(path)
}

fn write_echo_holon(
    root: &std::path::Path,
    binary_path: &std::path::Path,
) -> Result<(), Box<dyn Error>> {
    let holon_dir = root.join("holons/echo-server");
    fs::create_dir_all(&holon_dir)?;
    fs::write(
        holon_dir.join("holon.yaml"),
        format!(
            concat!(
                "uuid: \"echo-server-connect-example\"\n",
                "given_name: Echo\n",
                "family_name: Server\n",
                "motto: Reply precisely.\n",
                "composer: \"connect-example\"\n",
                "kind: service\n",
                "build:\n",
                "  runner: rust\n",
                "  main: bin/echo-server\n",
                "artifacts:\n",
                "  binary: \"{}\"\n"
            ),
            binary_path.display()
        ),
    )?;
    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let temp_root = TempDir::new()?;
    let echo_server = sdk_echo_server()?;
    write_echo_holon(temp_root.path(), &echo_server)?;
    let _cwd = CurrentDirGuard::enter(temp_root.path())?;

    let channel = holons::connect::connect("echo-server").await?;
    let mut grpc = Grpc::new(channel.clone());
    grpc.ready().await?;
    let response = grpc
        .unary(
            Request::new("{\"message\":\"hello-from-rust\"}".to_string()),
            PathAndQuery::from_static("/echo.v1.Echo/Ping"),
            StringCodec,
        )
        .await?
        .into_inner();

    println!("{response}");
    holons::connect::disconnect(channel).await?;
    Ok(())
}
