package transport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScheme(t *testing.T) {
	if got := Scheme("tcp://127.0.0.1:9090"); got != "tcp" {
		t.Fatalf("Scheme(tcp://...) = %q, want tcp", got)
	}
	if got := Scheme("stdio"); got != "stdio" {
		t.Fatalf("Scheme(stdio) = %q, want stdio", got)
	}
}

func TestListenVariantsAndUnsupported(t *testing.T) {
	tcpLis, err := Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	_ = tcpLis.Close()

	unixPath := filepath.Join(os.TempDir(), fmt.Sprintf("holon-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(unixPath) })
	unixLis, err := Listen("unix://" + unixPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	if _, statErr := os.Stat(unixPath); statErr != nil {
		t.Fatalf("expected unix socket file to exist: %v", statErr)
	}
	_ = unixLis.Close()

	stdioLis, err := Listen("stdio://")
	if err != nil {
		t.Fatalf("listen stdio: %v", err)
	}
	if stdioLis.Addr().String() != "stdio://" {
		t.Fatalf("stdio addr = %q, want stdio://", stdioLis.Addr().String())
	}
	_ = stdioLis.Close()

	wsLis, err := Listen("ws://127.0.0.1:0/grpc")
	if err != nil {
		t.Fatalf("listen ws: %v", err)
	}
	if !strings.HasPrefix(wsLis.Addr().String(), "ws://") {
		t.Fatalf("ws addr = %q, want ws:// prefix", wsLis.Addr().String())
	}
	_ = wsLis.Close()

	if _, err := Listen("wss://127.0.0.1:0/grpc"); err == nil {
		t.Fatal("expected wss listen error without cert/key")
	}

	if _, err := Listen("bad://host"); err == nil {
		t.Fatal("expected unsupported URI error")
	}
}

func TestWSListenerHTTPUpgradeFailure(t *testing.T) {
	lis, err := Listen("ws://127.0.0.1:0/grpc")
	if err != nil {
		t.Fatalf("listen ws: %v", err)
	}
	defer lis.Close()

	httpURL := "http://" + strings.TrimPrefix(lis.Addr().String(), "ws://")
	resp, err := http.Get(httpURL)
	if err != nil {
		t.Fatalf("http get %s: %v", httpURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf("status = %d, want %d or %d", resp.StatusCode, http.StatusBadRequest, http.StatusUpgradeRequired)
	}
}

func TestWSListenerAcceptAfterCloseReturnsEOF(t *testing.T) {
	lis, err := Listen("ws://127.0.0.1:0/grpc")
	if err != nil {
		t.Fatalf("listen ws: %v", err)
	}

	if err := lis.Close(); err != nil {
		t.Fatalf("close ws listener: %v", err)
	}

	_, err = lis.Accept()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("accept after close error = %v, want io.EOF", err)
	}
}

func TestStdioListenerAcceptAndClose(t *testing.T) {
	l := newStdioListener()

	conn, err := l.Accept()
	if err != nil {
		t.Fatalf("first accept: %v", err)
	}
	if conn.LocalAddr().String() != "stdio://" {
		t.Fatalf("conn local addr = %q, want stdio://", conn.LocalAddr().String())
	}
	if conn.RemoteAddr().Network() != "stdio" {
		t.Fatalf("conn remote network = %q, want stdio", conn.RemoteAddr().Network())
	}

	if err := l.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	_, err = l.Accept()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("accept after close error = %v, want io.EOF", err)
	}

	_ = conn.Close()
}

func TestStdioListenerAcceptClosedConnChan(t *testing.T) {
	l := &stdioListener{
		connCh: make(chan net.Conn),
		done:   make(chan struct{}),
	}
	close(l.connCh)

	_, err := l.Accept()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("accept on closed conn channel = %v, want io.EOF", err)
	}
}

func TestStdioConnMethods(t *testing.T) {
	var writer bytes.Buffer
	c := &stdioConn{
		reader: bytes.NewBufferString("abc"),
		writer: &writer,
	}

	buf := make([]byte, 3)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(buf[:n]); got != "abc" {
		t.Fatalf("read got %q, want abc", got)
	}

	if _, err := c.Write([]byte("xyz")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if writer.String() != "xyz" {
		t.Fatalf("writer got %q, want xyz", writer.String())
	}

	if err := c.SetDeadline(time.Now()); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if err := c.SetReadDeadline(time.Now()); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	if err := c.SetWriteDeadline(time.Now()); err != nil {
		t.Fatalf("set write deadline: %v", err)
	}

	if c.LocalAddr().Network() != "stdio" {
		t.Fatalf("local network = %q, want stdio", c.LocalAddr().Network())
	}
	if c.RemoteAddr().String() != "stdio://" {
		t.Fatalf("remote addr = %q, want stdio://", c.RemoteAddr().String())
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close twice: %v", err)
	}
}

func TestStdioCloseConcurrentDoesNotPanic(t *testing.T) {
	for i := 0; i < 2000; i++ {
		l := newStdioListener()
		conn, err := l.Accept()
		if err != nil {
			t.Fatalf("accept: %v", err)
		}

		done := make(chan struct{}, 2)
		go func() {
			_ = conn.Close()
			done <- struct{}{}
		}()
		go func() {
			_ = l.Close()
			done <- struct{}{}
		}()

		<-done
		<-done

		if err := conn.Close(); err != nil {
			t.Fatalf("close conn again: %v", err)
		}
		if err := l.Close(); err != nil {
			t.Fatalf("close listener again: %v", err)
		}
	}
}
