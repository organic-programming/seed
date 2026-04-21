"""Cross-language gRPC interop tests against a Go server."""

from __future__ import annotations

from contextlib import contextmanager
from pathlib import Path
import shutil
import ssl
import subprocess
import tempfile
import textwrap
from typing import Iterator

import pytest
from grpc_reflection.v1alpha import reflection_pb2, reflection_pb2_grpc

from holons.grpcclient import dial_uri


GO_GRPC_REFLECTION_SERVER = r'''
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	reflection.Register(srv)

	go func() {
		_ = srv.Serve(lis)
	}()

	fmt.Printf("tcp://%s\n", lis.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		srv.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = ctx
}
'''


GO_GRPC_REFLECTION_WS_SERVER = r'''
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	lis, err := transport.Listen("ws://127.0.0.1:0/grpc")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	reflection.Register(srv)

	go func() {
		_ = srv.Serve(lis)
	}()

	fmt.Printf("%s\n", lis.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		srv.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = ctx
}
'''


GO_GRPC_REFLECTION_WSS_SERVER = r'''
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/organic-programming/go-holons/pkg/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	certFile, keyFile, err := writeSelfSignedCert()
	if err != nil {
		panic(err)
	}
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	lis, err := transport.Listen(fmt.Sprintf(
		"wss://127.0.0.1:0/grpc?cert=%s&key=%s",
		url.QueryEscape(certFile),
		url.QueryEscape(keyFile),
	))
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	reflection.Register(srv)

	go func() {
		_ = srv.Serve(lis)
	}()

	fmt.Printf("%s\n", lis.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	done := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		srv.Stop()
	}
}

func writeSelfSignedCert() (string, string, error) {
	dir, err := os.MkdirTemp("", "python-holons-wss-*")
	if err != nil {
		return "", "", err
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", err
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{CommonName: "127.0.0.1"},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:  []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}

	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return "", "", err
	}

	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", "", err
	}
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return "", "", err
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return "", "", err
	}

	return certFile, keyFile, nil
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
def _run_go_grpc_server() -> Iterator[str]:
    go_bin = _resolve_go_binary()
    sdk_dir = Path(__file__).resolve().parents[2]
    go_holons_dir = sdk_dir / "go-holons"

    with tempfile.NamedTemporaryFile("w", suffix=".go", dir=go_holons_dir, delete=False) as f:
        f.write(textwrap.dedent(GO_GRPC_REFLECTION_SERVER))
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
        uri = proc.stdout.readline().strip()
        if not uri:
            stderr = ""
            if proc.stderr is not None:
                stderr = proc.stderr.read()
            if _is_bind_denied(stderr):
                pytest.skip("local bind denied in this environment")
            raise RuntimeError(f"failed to start Go gRPC helper: {stderr}")
        yield uri
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
        helper_path.unlink(missing_ok=True)


@contextmanager
def _run_go_grpc_ws_server() -> Iterator[str]:
    go_bin = _resolve_go_binary()
    sdk_dir = Path(__file__).resolve().parents[2]
    go_holons_dir = sdk_dir / "go-holons"

    with tempfile.NamedTemporaryFile("w", suffix=".go", dir=go_holons_dir, delete=False) as f:
        f.write(textwrap.dedent(GO_GRPC_REFLECTION_WS_SERVER))
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
        uri = proc.stdout.readline().strip()
        if not uri:
            stderr = ""
            if proc.stderr is not None:
                stderr = proc.stderr.read()
            if _is_bind_denied(stderr):
                pytest.skip("local bind denied in this environment")
            raise RuntimeError(f"failed to start Go gRPC ws helper: {stderr}")
        yield uri
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
        helper_path.unlink(missing_ok=True)


@contextmanager
def _run_go_grpc_wss_server() -> Iterator[str]:
    go_bin = _resolve_go_binary()
    sdk_dir = Path(__file__).resolve().parents[2]
    go_holons_dir = sdk_dir / "go-holons"

    with tempfile.NamedTemporaryFile("w", suffix=".go", dir=go_holons_dir, delete=False) as f:
        f.write(textwrap.dedent(GO_GRPC_REFLECTION_WSS_SERVER))
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
        uri = proc.stdout.readline().strip()
        if not uri:
            stderr = ""
            if proc.stderr is not None:
                stderr = proc.stderr.read()
            if _is_bind_denied(stderr):
                pytest.skip("local bind denied in this environment")
            raise RuntimeError(f"failed to start Go gRPC wss helper: {stderr}")
        yield uri
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)
        helper_path.unlink(missing_ok=True)


def test_grpc_cross_language_reflection_roundtrip():
    with _run_go_grpc_server() as uri:
        ch = dial_uri(uri)
        try:
            stub = reflection_pb2_grpc.ServerReflectionStub(ch)
            req = reflection_pb2.ServerReflectionRequest(
                list_services="",
            )
            stream = stub.ServerReflectionInfo(iter([req]))
            resp = next(stream)
            services = [svc.name for svc in resp.list_services_response.service]
            assert "grpc.reflection.v1alpha.ServerReflection" in services
        finally:
            ch.close()


def test_grpc_cross_language_reflection_roundtrip_ws():
    with _run_go_grpc_ws_server() as uri:
        ch = dial_uri(uri)
        try:
            stub = reflection_pb2_grpc.ServerReflectionStub(ch)
            req = reflection_pb2.ServerReflectionRequest(
                list_services="",
            )
            stream = stub.ServerReflectionInfo(iter([req]))
            resp = next(stream)
            services = [svc.name for svc in resp.list_services_response.service]
            assert "grpc.reflection.v1alpha.ServerReflection" in services
        finally:
            ch.close()


def test_grpc_cross_language_reflection_roundtrip_wss():
    with _run_go_grpc_wss_server() as uri:
        ch = dial_uri(uri, websocket_ssl_context=ssl._create_unverified_context())
        try:
            stub = reflection_pb2_grpc.ServerReflectionStub(ch)
            req = reflection_pb2.ServerReflectionRequest(
                list_services="",
            )
            stream = stub.ServerReflectionInfo(iter([req]))
            resp = next(stream)
            services = [svc.name for svc in resp.list_services_response.service]
            assert "grpc.reflection.v1alpha.ServerReflection" in services
        finally:
            ch.close()
