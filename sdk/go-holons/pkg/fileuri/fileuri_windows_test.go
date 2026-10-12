//go:build windows

package fileuri

import "testing"

func TestWindowsDriveAndUNCFileURIs(t *testing.T) {
	for _, tc := range []struct{ path, uri string }{
		{`C:\Users\benoit\project #1`, "file:///C:/Users/benoit/project%20%231"},
		{`\\server\share\project #1`, "file://server/share/project%20%231"},
	} {
		if got := FromPath(tc.path); got != tc.uri {
			t.Errorf("FromPath(%q) = %q, want %q", tc.path, got, tc.uri)
		}
		if got, err := ToPath(tc.uri); err != nil || got != tc.path {
			t.Errorf("ToPath(%q) = %q, %v; want %q", tc.uri, got, err, tc.path)
		}
	}
	for _, uri := range []string{"file://C:/Users/benoit/.op/bin", "file:///C:/Users/benoit/.op/bin", "file://localhost/C:/Users/benoit/.op/bin"} {
		if got, err := ToPath(uri); err != nil || got != `C:\Users\benoit\.op\bin` {
			t.Errorf("ToPath(%q) = %q, %v", uri, got, err)
		}
	}
}
