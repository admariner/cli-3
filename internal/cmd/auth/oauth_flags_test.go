package auth

import (
	"bytes"
	"strings"
	"testing"

	psauth "github.com/planetscale/cli/internal/auth"
	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func TestAuthLoginHelpOmitsOAuthSecrets(t *testing.T) {
	assertHelpOmitsOAuthSecrets(t, LoginCmd(&cmdutil.Helper{}))
}

func TestAuthLogoutHelpOmitsOAuthSecrets(t *testing.T) {
	assertHelpOmitsOAuthSecrets(t, LogoutCmd(&cmdutil.Helper{}))
}

func TestAuthCheckHelpOmitsOAuthSecrets(t *testing.T) {
	assertHelpOmitsOAuthSecrets(t, CheckCmd(&cmdutil.Helper{}))
}

func assertHelpOmitsOAuthSecrets(t *testing.T, cmd *cobra.Command) {
	t.Helper()

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help: %v", err)
	}

	help := out.String()
	if strings.Contains(help, psauth.OAuthClientID) {
		t.Fatalf("help text must not include OAuth client ID; got:\n%s", help)
	}
	if strings.Contains(help, psauth.OAuthClientSecret) {
		t.Fatalf("help text must not include OAuth client secret; got:\n%s", help)
	}
	if !strings.Contains(help, "--client-id") {
		t.Fatalf("help text should still document --client-id; got:\n%s", help)
	}
	if !strings.Contains(help, "--client-secret") {
		t.Fatalf("help text should still document --client-secret; got:\n%s", help)
	}
}

func TestResolveOAuthClientDefaults(t *testing.T) {
	id, secret := resolveOAuthClient("", "")
	if id != psauth.OAuthClientID {
		t.Fatalf("client ID = %q, want built-in default", id)
	}
	if secret != psauth.OAuthClientSecret {
		t.Fatalf("client secret = %q, want built-in default", secret)
	}
}

func TestResolveOAuthClientOverrides(t *testing.T) {
	id, secret := resolveOAuthClient("custom-id", "custom-secret")
	if id != "custom-id" {
		t.Fatalf("client ID = %q, want custom-id", id)
	}
	if secret != "custom-secret" {
		t.Fatalf("client secret = %q, want custom-secret", secret)
	}
}
