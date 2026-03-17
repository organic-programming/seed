pub mod gen {
    pub mod rust {
        pub mod greeting {
            pub mod v1 {
                include!(concat!(
                    env!("CARGO_MANIFEST_DIR"),
                    "/gen/rust/greeting/v1/greeting.v1.rs"
                ));
            }
        }
    }
}

#[path = "../api/cli.rs"]
pub mod cli;
#[path = "../internal/mod.rs"]
mod internal;
#[path = "../api/public.rs"]
pub mod public;

#[cfg(test)]
#[path = "../api/cli_test.rs"]
mod cli_test;
#[cfg(test)]
#[path = "../api/public_test.rs"]
mod public_test;
#[cfg(test)]
#[path = "../internal/server_test.rs"]
mod server_test;
