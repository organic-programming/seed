package observability

import (
	"context"
	"net"
	"runtime"
	"testing"
)

func TestPromServerPreservesExplicitAddressWithoutBinding(t *testing.T) {
	for _, address := range []string{"127.0.0.2:0", "0.0.0.0:0", ":0"} {
		t.Run(address, func(t *testing.T) {
			server := NewPromServer(address)
			if server.addr != address {
				t.Fatalf("configured address = %q, want %q", server.addr, address)
			}
			if server.lis != nil {
				t.Fatal("constructing a server must not bind a socket")
			}
		})
	}
}

func TestPromServerDefaultAndExplicitBindings(t *testing.T) {
	for _, tc := range []struct {
		name        string
		address     string
		wantIP      string
		unspecified bool
	}{
		{name: "default-loopback", wantIP: "127.0.0.1"},
		{name: "explicit-loopback", address: "127.0.0.2:0", wantIP: "127.0.0.2"},
		{name: "explicit-wildcard", address: "0.0.0.0:0", unspecified: true},
		{name: "explicit-port-only", address: ":0", unspecified: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.unspecified && runtime.GOOS == "windows" {
				t.Skip("wildcard socket binding is skipped on Windows to avoid firewall authorization prompts; explicit configuration is tested without binding")
			}
			server := NewPromServer(tc.address)
			if _, err := server.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Close(context.Background()) })
			// Inspect the actual socket, not an advertised URL that may rewrite
			// wildcard addresses to a loopback client destination.
			bound := server.lis.Addr().(*net.TCPAddr)
			if bound.Port == 0 {
				t.Fatal("listener did not acquire an ephemeral port")
			}
			if tc.unspecified {
				if !bound.IP.IsUnspecified() {
					t.Fatalf("bound to %v, want an explicitly requested wildcard", bound)
				}
			} else if !bound.IP.Equal(net.ParseIP(tc.wantIP)) {
				t.Fatalf("bound to %v, want %s", bound, tc.wantIP)
			}
		})
	}
}
