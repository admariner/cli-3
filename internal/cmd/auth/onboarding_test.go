package auth

import (
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
)

func TestBuildAuthCheckResponseNoAuth(t *testing.T) {
	ch := &cmdutil.Helper{
		Config: &config.Config{BaseURL: "https://api.example.com/v1"},
	}
	resp := buildAuthCheckResponse(t.Context(), ch)
	if resp.Authenticated {
		t.Fatal("expected unauthenticated")
	}
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if len(resp.Issues) == 0 || resp.Issues[0].Code != "NO_AUTH" {
		t.Fatalf("issues = %#v", resp.Issues)
	}
}

func TestLoginPendingMessage(t *testing.T) {
	if got := loginPendingMessage(true); got != "Approve access in the browser to continue" {
		t.Fatalf("browser opened message = %q", got)
	}
	if got := loginPendingMessage(false); got != "Open verification_url in a browser and approve access to continue" {
		t.Fatalf("browser closed message = %q", got)
	}
}

func TestConfiguredOrganization(t *testing.T) {
	ch := &cmdutil.Helper{
		Config: &config.Config{Organization: "from-flag"},
	}
	if got := configuredOrganization(ch); got != "from-flag" {
		t.Fatalf("org = %q", got)
	}
}
