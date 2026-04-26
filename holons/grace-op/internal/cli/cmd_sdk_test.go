package cli

import "testing"

func TestSDKInstallRequestFromCommand(t *testing.T) {
	cmd := newSdkInstallCmd()
	if err := cmd.Flags().Set("target", "x86_64-unknown-linux-gnu"); err != nil {
		t.Fatalf("set target: %v", err)
	}
	if err := cmd.Flags().Set("version", "1.80.0"); err != nil {
		t.Fatalf("set version: %v", err)
	}
	if err := cmd.Flags().Set("source", "/tmp/cpp.tar.gz"); err != nil {
		t.Fatalf("set source: %v", err)
	}

	req := sdkInstallRequestFromCommand(cmd, " cpp ")
	if req.GetLang() != "cpp" {
		t.Fatalf("lang = %q, want cpp", req.GetLang())
	}
	if req.GetTarget() != "x86_64-unknown-linux-gnu" {
		t.Fatalf("target = %q", req.GetTarget())
	}
	if req.GetVersion() != "1.80.0" {
		t.Fatalf("version = %q", req.GetVersion())
	}
	if req.GetSource() != "/tmp/cpp.tar.gz" {
		t.Fatalf("source = %q", req.GetSource())
	}
}

func TestSDKListRequestDefaultsToInstalled(t *testing.T) {
	cmd := newSdkListCmd()

	req := sdkListRequestFromCommand(cmd)
	if !req.GetInstalled() {
		t.Fatalf("installed = false, want true")
	}
	if req.GetAvailable() {
		t.Fatalf("available = true, want false")
	}
}

func TestSDKListRequestAllowsAvailableFilter(t *testing.T) {
	cmd := newSdkListCmd()
	if err := cmd.Flags().Set("installed", "false"); err != nil {
		t.Fatalf("set installed: %v", err)
	}
	if err := cmd.Flags().Set("available", "true"); err != nil {
		t.Fatalf("set available: %v", err)
	}
	if err := cmd.Flags().Set("lang", " zig "); err != nil {
		t.Fatalf("set lang: %v", err)
	}

	req := sdkListRequestFromCommand(cmd)
	if req.GetInstalled() {
		t.Fatalf("installed = true, want false")
	}
	if !req.GetAvailable() {
		t.Fatalf("available = false, want true")
	}
	if req.GetLang() != "zig" {
		t.Fatalf("lang = %q, want zig", req.GetLang())
	}
}

func TestSDKQueryRequestsFromCommand(t *testing.T) {
	for _, tc := range []struct {
		name string
		cmd  func() commandWithQuery
	}{
		{name: "uninstall", cmd: func() commandWithQuery {
			cmd := newSdkUninstallCmd()
			return commandWithQuery{
				setTarget: cmd.Flags().Set,
				request: func() (string, string, string) {
					req := sdkUninstallRequestFromCommand(cmd, "ruby")
					return req.GetLang(), req.GetTarget(), req.GetVersion()
				},
			}
		}},
		{name: "verify", cmd: func() commandWithQuery {
			cmd := newSdkVerifyCmd()
			return commandWithQuery{
				setTarget: cmd.Flags().Set,
				request: func() (string, string, string) {
					req := sdkVerifyRequestFromCommand(cmd, "ruby")
					return req.GetLang(), req.GetTarget(), req.GetVersion()
				},
			}
		}},
		{name: "path", cmd: func() commandWithQuery {
			cmd := newSdkPathCmd()
			return commandWithQuery{
				setTarget: cmd.Flags().Set,
				request: func() (string, string, string) {
					req := sdkLocateRequestFromCommand(cmd, "ruby")
					return req.GetLang(), req.GetTarget(), req.GetVersion()
				},
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query := tc.cmd()
			if err := query.setTarget("target", "aarch64-apple-darwin"); err != nil {
				t.Fatalf("set target: %v", err)
			}
			if err := query.setTarget("version", "1.58.3"); err != nil {
				t.Fatalf("set version: %v", err)
			}
			lang, target, version := query.request()
			if lang != "ruby" || target != "aarch64-apple-darwin" || version != "1.58.3" {
				t.Fatalf("request = (%q, %q, %q)", lang, target, version)
			}
		})
	}
}

type commandWithQuery struct {
	setTarget func(string, string) error
	request   func() (string, string, string)
}
