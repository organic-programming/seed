use tonic::transport::Channel;

pub const LOCAL: i32 = 0;
pub const PROXY: i32 = 1;
pub const DELEGATED: i32 = 2;

pub const SIBLINGS: i32 = 0x01;
pub const CWD: i32 = 0x02;
pub const SOURCE: i32 = 0x04;
pub const BUILT: i32 = 0x08;
pub const INSTALLED: i32 = 0x10;
pub const CACHED: i32 = 0x20;
pub const ALL: i32 = 0x3F;

pub const NO_LIMIT: i32 = 0;
pub const NO_TIMEOUT: u32 = 0;

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct IdentityInfo {
    pub given_name: String,
    pub family_name: String,
    pub motto: String,
    pub aliases: Vec<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct HolonInfo {
    pub slug: String,
    pub uuid: String,
    pub identity: IdentityInfo,
    pub lang: String,
    pub runner: String,
    pub status: String,
    pub kind: String,
    pub transport: String,
    pub entrypoint: String,
    pub architectures: Vec<String>,
    pub has_dist: bool,
    pub has_source: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct HolonRef {
    pub url: String,
    pub info: Option<HolonInfo>,
    pub error: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct DiscoverResult {
    pub found: Vec<HolonRef>,
    pub error: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ResolveResult {
    pub r#ref: Option<HolonRef>,
    pub error: Option<String>,
}

#[derive(Debug)]
pub struct ConnectResult {
    pub channel: Option<Channel>,
    pub uid: String,
    pub origin: Option<HolonRef>,
    pub error: Option<String>,
}
