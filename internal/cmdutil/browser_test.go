package cmdutil

import (
	"strings"
	"testing"
)

func TestValidateBrowserURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{name: "https", url: "https://auth.planetscale.com/device?user_code=ABC"},
		{name: "http", url: "http://example.com/device"},
		{name: "https uppercase scheme", url: "HTTPS://example.com/ok"},
		{name: "empty", url: "", wantErr: "must not be empty"},
		{name: "leading dash", url: "-https://example.com", wantErr: "must not start with '-'"},
		{name: "file scheme", url: "file:///etc/passwd", wantErr: "unsupported URL scheme"},
		{name: "javascript scheme", url: "javascript:alert(1)", wantErr: "unsupported URL scheme"},
		{name: "missing scheme", url: "example.com/path", wantErr: "unsupported URL scheme"},
		{name: "missing host", url: "https:///path", wantErr: "must include a host"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateBrowserURL(tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateBrowserURL(%q) = %v, want nil", tt.url, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateBrowserURL(%q) = nil, want error containing %q", tt.url, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateBrowserURL(%q) = %v, want error containing %q", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestBrowserCommandWindowsUsesRundll32(t *testing.T) {
	t.Parallel()

	// PoC-style URL with cmd.exe metacharacters. Must not go through `cmd /c start`.
	rawURL := "https://example.com/device?user_code=ABC&x=1|calc.exe"
	cmd, err := browserCommand("windows", rawURL)
	if err != nil {
		t.Fatalf("browserCommand: %v", err)
	}
	if cmd.Args[0] != "rundll32" {
		t.Fatalf("Args[0] = %q, want rundll32", cmd.Args[0])
	}
	wantArgs := []string{"rundll32", "url.dll,FileProtocolHandler", rawURL}
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("Args = %#v, want %#v", cmd.Args, wantArgs)
	}
	for i, want := range wantArgs {
		if cmd.Args[i] != want {
			t.Fatalf("Args[%d] = %q, want %q", i, cmd.Args[i], want)
		}
	}
}

func TestBrowserCommandDarwinAndLinuxPassURLArg(t *testing.T) {
	t.Parallel()

	rawURL := "https://example.com/login"
	for _, goos := range []string{"darwin", "linux"} {
		cmd, err := browserCommand(goos, rawURL)
		if err != nil {
			t.Fatalf("%s: %v", goos, err)
		}
		if len(cmd.Args) < 2 || cmd.Args[len(cmd.Args)-1] != rawURL {
			t.Fatalf("%s Args = %#v, want trailing %q", goos, cmd.Args, rawURL)
		}
	}
}

func TestTryOpenBrowserRejectsUnsafeURL(t *testing.T) {
	t.Parallel()

	err := TryOpenBrowser("linux", "file:///tmp/x")
	if err == nil {
		t.Fatal("TryOpenBrowser(file:) = nil, want error")
	}

	err = TryOpenBrowser("windows", "javascript:alert(1)")
	if err == nil {
		t.Fatal("TryOpenBrowser(javascript:) = nil, want error")
	}

	err = TryOpenBrowser("darwin", "-https://example.com")
	if err == nil {
		t.Fatal("TryOpenBrowser(leading dash) = nil, want error")
	}
}
