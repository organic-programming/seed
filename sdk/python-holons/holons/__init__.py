"""holons — Organic Programming SDK for Python."""

from importlib import import_module
from pathlib import Path
from pkgutil import extend_path
import sys

__path__ = extend_path(__path__, __name__)

_GENERATED_PACKAGE_DIR = Path(__file__).resolve().parents[1] / "gen" / "python" / "holons"
_GENERATED_ROOT_DIR = Path(__file__).resolve().parents[1] / "gen" / "python"
if _GENERATED_PACKAGE_DIR.is_dir():
    generated_path = str(_GENERATED_PACKAGE_DIR)
    if generated_path not in __path__:
        __path__.append(generated_path)
if _GENERATED_ROOT_DIR.is_dir():
    generated_root = str(_GENERATED_ROOT_DIR)
    if generated_root not in sys.path:
        sys.path.insert(0, generated_root)

from .connect import connect, disconnect
from .composite import member
from .discover import Discover, resolve
from .discovery_types import (
    ALL,
    BUILT,
    CACHED,
    CWD,
    DELEGATED,
    INSTALLED,
    LOCAL,
    NO_LIMIT,
    NO_TIMEOUT,
    PROXY,
    SIBLINGS,
    SOURCE,
    ConnectResult,
    DiscoverResult,
    HolonInfo,
    HolonRef,
    IdentityInfo,
    ResolveResult,
)

_MODULE_EXPORTS = [
    "transport",
    "serve",
    "identity",
    "discover",
    "grpcclient",
    "holonrpc",
    "describe",
    "composite",
    "relay",
]

__all__ = [
    "Discover",
    "resolve",
    "connect",
    "disconnect",
    "member",
    "LOCAL",
    "PROXY",
    "DELEGATED",
    "SIBLINGS",
    "CWD",
    "SOURCE",
    "BUILT",
    "INSTALLED",
    "CACHED",
    "ALL",
    "NO_LIMIT",
    "NO_TIMEOUT",
    "IdentityInfo",
    "HolonInfo",
    "HolonRef",
    "DiscoverResult",
    "ResolveResult",
    "ConnectResult",
    *_MODULE_EXPORTS,
]


def __getattr__(name: str):
    if name in _MODULE_EXPORTS:
        module = import_module(f"{__name__}.{name}")
        globals()[name] = module
        return module
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
