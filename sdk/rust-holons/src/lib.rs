//! holons — Organic Programming SDK for Rust
//!
//! Transport, serve, and identity utilities for building holons in Rust.

mod command_channel;
pub mod connect;
pub mod describe;
pub mod discover;
pub mod discovery_types;
pub mod holonrpc;
pub mod gen {
    pub mod holons {
        pub mod v1 {
            include!("gen/holons.v1.rs");
        }
    }
}
pub mod identity;
pub mod observability;
pub mod serve;
pub mod transport;

pub use connect::{connect, disconnect};
pub use discover::{discover, resolve};
pub use discovery_types::{
    ConnectResult, DiscoverResult, HolonInfo, HolonRef, IdentityInfo, ResolveResult, ALL, BUILT,
    CACHED, CWD, DELEGATED, INSTALLED, LOCAL, NO_LIMIT, NO_TIMEOUT, PROXY, SIBLINGS, SOURCE,
};

#[cfg(test)]
pub(crate) mod test_support;
