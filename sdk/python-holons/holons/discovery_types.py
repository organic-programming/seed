from __future__ import annotations

"""Uniform discovery API result types and constants."""

from dataclasses import dataclass, field

import grpc

# Scope flags.
LOCAL = 0
PROXY = 1
DELEGATED = 2

# Layer flags.
SIBLINGS = 0x01
CWD = 0x02
SOURCE = 0x04
BUILT = 0x08
INSTALLED = 0x10
CACHED = 0x20
ALL = 0x3F

# Clarity constants.
NO_LIMIT = 0
NO_TIMEOUT = 0


@dataclass
class IdentityInfo:
    given_name: str
    family_name: str
    motto: str = ""
    aliases: list[str] = field(default_factory=list)


@dataclass
class HolonInfo:
    slug: str
    uuid: str
    identity: IdentityInfo
    lang: str
    runner: str
    status: str
    kind: str
    transport: str
    entrypoint: str
    architectures: list[str]
    has_dist: bool
    has_source: bool


@dataclass
class HolonRef:
    url: str
    info: HolonInfo | None
    error: str | None


@dataclass
class DiscoverResult:
    found: list[HolonRef]
    error: str | None


@dataclass
class ResolveResult:
    ref: HolonRef | None
    error: str | None


@dataclass
class ConnectResult:
    channel: grpc.Channel | None
    uid: str
    origin: HolonRef | None
    error: str | None
