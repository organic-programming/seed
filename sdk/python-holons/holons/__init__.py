"""holons — Organic Programming SDK for Python."""

from importlib import import_module
from pathlib import Path
from pkgutil import extend_path

__path__ = extend_path(__path__, __name__)

_GENERATED_PACKAGE_DIR = Path(__file__).resolve().parents[1] / "gen" / "python" / "holons"
if _GENERATED_PACKAGE_DIR.is_dir():
    generated_path = str(_GENERATED_PACKAGE_DIR)
    if generated_path not in __path__:
        __path__.append(generated_path)

__all__ = ["transport", "serve", "identity", "discover", "connect", "grpcclient", "holonrpc", "describe"]


def __getattr__(name: str):
    if name in __all__:
        module = import_module(f"{__name__}.{name}")
        globals()[name] = module
        return module
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
