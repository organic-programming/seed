"""Interop tests for Holon-RPC HTTP+SSE client against a Go server."""

from __future__ import annotations

from contextlib import contextmanager
from pathlib import Path
import shutil
import subprocess
import tempfile
import textwrap
from typing import Iterator

import pytest

from holons.holonrpc import HTTPClient


HOLONRPC_HTTP_SERVER_GO = r'''
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/holonrpc"
)

func main() {
	server := holonrpc.NewHTTPServer("http://127.0.0.1:0/api/v1/rpc")
	server.Register("echo.v1.Echo/Ping", func(_ context.Context, params map[string]any) (map[string]any, error) {
		return params, nil
	})
	server.RegisterStream("build.v1.Build/Watch", func(_ context.Context, params map[string]any, send func(map[string]any) error) error {
		project := fmt.Sprintf("%v", params["project"])
		if err := send(map[string]any{"status": "building", "project": project}); err != nil {
			return err
		}
		return send(map[string]any{"status": "done", "project": project})
	})

	addr, err := server.Start()
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Close(ctx)
}
'''


def _resolve_go_binary() -> str:
    preferred = Path("/Users/bpds/go/go1.25.1/bin/go")
    if preferred.exists():
        return str(preferred)
    found = shutil.which("go")
    if not found:
        raise RuntimeError("go binary not found")
    return found


def _is_bind_denied(stderr: str) -> bool:
    text = stderr.lower()
    return "bind" in text and "operation not permitted" in text


@contextmanager
def _run_go_holonrpc_http_server() -> Iterator[str]:
    go_bin = _resolve_go_binary()
    sdk_dir = Path(__file__).resolve().parents[2]
    go_holons_dir = sdk_dir / "go-holons"

    with tempfile.NamedTemporaryFile("w", suffix=".go", dir=go_holons_dir, delete=False) as f:
        f.write(textwrap.dedent(HOLONRPC_HTTP_SERVER_GO))
        helper_path = Path(f.name)

    proc = subprocess.Popen(
        [go_bin, "run", str(helper_path)],
        cwd=go_holons_dir,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )

    try:
        assert proc.stdout is not None
        url = proc.stdout.readline().strip()
        if not url:
            stderr = ""
            if proc.stderr is not None:
                stderr = proc.stderr.read()
            if _is_bind_denied(stderr):
                pytest.skip("local bind denied in this environment")
            raise RuntimeError(f"failed to start Go holon-rpc http helper: {stderr}")
        yield url
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
        helper_path.unlink(missing_ok=True)


def test_holonrpc_http_invoke_roundtrip():
    with _run_go_holonrpc_http_server() as url:
        client = HTTPClient(url.replace("http://", "rest+sse://", 1))
        out = client.invoke("echo.v1.Echo/Ping", {"message": "hello-http"})
        assert out["message"] == "hello-http"


def test_holonrpc_http_stream_post_and_query():
    with _run_go_holonrpc_http_server() as url:
        client = HTTPClient(url)

        events = client.stream("build.v1.Build/Watch", {"project": "myapp"})
        assert [event.event for event in events] == ["message", "message", "done"]
        assert events[0].result["status"] == "building"
        assert events[1].result["status"] == "done"

        query_events = client.stream_query("build.v1.Build/Watch", {"project": "myapp"})
        assert [event.event for event in query_events] == ["message", "message", "done"]
        assert query_events[0].result["project"] == "myapp"
