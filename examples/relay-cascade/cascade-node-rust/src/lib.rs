pub mod gen {
    pub mod describe_generated {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/gen/describe_generated.rs"
        ));
    }

    pub mod rust {
        pub mod relay {
            pub mod v1 {
                include!(concat!(
                    env!("CARGO_MANIFEST_DIR"),
                    "/gen/rust/relay/v1/relay.v1.rs"
                ));
            }
        }
    }
}

#[path = "../api/cli.rs"]
pub mod cli;
#[path = "../internal/server.rs"]
mod server;
