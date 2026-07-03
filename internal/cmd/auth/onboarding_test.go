package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/planetscale/cli/internal/cmdutil"
	"github.com/planetscale/cli/internal/config"
	"github.com/planetscale/cli/internal/printer"
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

func TestAuthCheckExitCode(t *testing.T) {
	if err := authCheckExitCode(AuthCheckResponse{Status: "ok"}); err != nil {
		t.Fatalf("ok status: %v", err)
	}
	if err := authCheckExitCode(AuthCheckResponse{Status: "action_required", Authenticated: true}); err == nil {
		t.Fatal("expected non-zero exit for action_required")
	}
}

func TestInvalidAuthIssueAndNextStepsServiceToken(t *testing.T) {
	issue, nextSteps := invalidAuthIssueAndNextSteps("service_token")
	if issue.Code != "SERVICE_TOKEN_INVALID" {
		t.Fatalf("code = %q", issue.Code)
	}
	if len(nextSteps) != 1 || nextSteps[0] != cmdutil.AgentAuthCheckCmd() {
		t.Fatalf("next steps = %#v", nextSteps)
	}
}

func TestInvalidAuthIssueAndNextStepsOAuth(t *testing.T) {
	issue, nextSteps := invalidAuthIssueAndNextSteps("oauth")
	if issue.Code != "AUTH_INVALID" {
		t.Fatalf("code = %q", issue.Code)
	}
	if len(nextSteps) != 1 || nextSteps[0] != cmdutil.AgentAuthLoginCmd() {
		t.Fatalf("next steps = %#v", nextSteps)
	}
}

func TestFinishLoginJSONOrgSetupFailure(t *testing.T) {
	format := printer.JSON
	var out bytes.Buffer
	ch := &cmdutil.Helper{
		Printer: printer.NewPrinter(&format),
	}
	ch.Printer.SetResourceOutput(&out)

	err := finishLoginJSON(ch, errors.New("org list unavailable"))
	if err == nil {
		t.Fatal("expected exit error")
	}
	var cmdErr *cmdutil.Error
	if !errors.As(err, &cmdErr) || !cmdErr.Handled {
		t.Fatalf("expected handled JSON error, got %v", err)
	}

	var resp LoginOKResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("stdout json: %v", err)
	}
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if len(resp.Issues) == 0 || resp.Issues[0].Code != "ORG_SETUP_FAILED" {
		t.Fatalf("issues = %#v", resp.Issues)
	}
}
