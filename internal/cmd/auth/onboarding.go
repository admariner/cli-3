package auth

import (
	"context"

	"github.com/planetscale/cli/internal/cmdutil"
)

// AuthIssue describes a blocking onboarding problem for agent/human callers.
type AuthIssue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

// AuthCheckResponse is the JSON payload for `pscale auth check --format json`.
type AuthCheckResponse struct {
	Status        string      `json:"status"`
	Authenticated bool        `json:"authenticated"`
	AuthMethod    string      `json:"auth_method,omitempty"`
	Organization  string      `json:"organization,omitempty"`
	APIURL        string      `json:"api_url,omitempty"`
	Issues        []AuthIssue `json:"issues,omitempty"`
	NextSteps     []string    `json:"next_steps,omitempty"`
}

func buildAuthCheckResponse(ctx context.Context, ch *cmdutil.Helper) AuthCheckResponse {
	resp := AuthCheckResponse{
		APIURL: ch.Config.BaseURL,
	}

	org := configuredOrganization(ch)
	if org != "" {
		resp.Organization = org
	}

	switch {
	case ch.Config.ServiceTokenIsSet():
		resp.AuthMethod = "service_token"
	case ch.Config.AccessToken != "":
		resp.AuthMethod = "oauth"
	default:
		resp.AuthMethod = "none"
	}

	if err := ch.Config.IsAuthenticated(); err != nil {
		resp.Status = "action_required"
		resp.Authenticated = false
		resp.Issues = append(resp.Issues, AuthIssue{
			Code:        "NO_AUTH",
			Message:     err.Error(),
			Remediation: "Run `pscale auth login --format json` (browser opens when possible, polls until approved)",
		})
		resp.NextSteps = []string{
			cmdutil.AgentAuthLoginCmd(),
			cmdutil.AgentAuthCheckCmd(),
		}
		return resp
	}

	client, err := ch.Client()
	if err != nil {
		resp.Status = "action_required"
		resp.Authenticated = false
		resp.Issues = append(resp.Issues, AuthIssue{
			Code:        "CLIENT_INIT_FAILED",
			Message:     err.Error(),
			Remediation: "Verify API credentials and network connectivity",
		})
		return resp
	}

	if _, err := client.Organizations.List(ctx); err != nil {
		resp.Status = "action_required"
		resp.Authenticated = false
		resp.Issues = append(resp.Issues, AuthIssue{
			Code:        "AUTH_INVALID",
			Message:     "API authentication failed",
			Remediation: "Run `pscale auth login --format json` (browser opens when possible)",
		})
		resp.NextSteps = []string{cmdutil.AgentAuthLoginCmd()}
		return resp
	}

	resp.Status = "ok"
	resp.Authenticated = true

	if org == "" {
		resp.Issues = append(resp.Issues, AuthIssue{
			Code:        "NO_ORG",
			Message:     "No organization configured",
			Remediation: "Run `pscale org switch <org>` or set org in ~/.config/planetscale/pscale.yml",
		})
		resp.NextSteps = append(resp.NextSteps, cmdutil.AgentOrgListCmd())
	} else {
		resp.NextSteps = append(resp.NextSteps,
			cmdutil.AgentDatabaseListCmd(org),
			cmdutil.AgentBranchListCmd(org, "<database>"),
		)
	}

	if len(resp.Issues) > 0 {
		resp.Status = "action_required"
	}

	return resp
}

func configuredOrganization(ch *cmdutil.Helper) string {
	if ch.Config.Organization != "" {
		return ch.Config.Organization
	}
	if ch.ConfigFS == nil {
		return ""
	}
	if fc, err := ch.ConfigFS.DefaultConfig(); err == nil && fc.Organization != "" {
		return fc.Organization
	}
	if fc, err := ch.ConfigFS.ProjectConfig(); err == nil && fc.Organization != "" {
		return fc.Organization
	}
	return ""
}

func loginPendingMessage(browserOpened bool) string {
	if browserOpened {
		return "Approve access in the browser to continue"
	}
	return "Open verification_url in a browser and approve access to continue"
}

// LoginPendingResponse is emitted while waiting for browser authorization.
type LoginPendingResponse struct {
	Status          string   `json:"status"`
	VerificationURL string   `json:"verification_url"`
	UserCode        string   `json:"user_code"`
	BrowserOpened   bool     `json:"browser_opened"`
	Message         string   `json:"message"`
	NextSteps       []string `json:"next_steps,omitempty"`
}

// LoginOKResponse is emitted after successful login.
type LoginOKResponse struct {
	Status    string   `json:"status"`
	Message   string   `json:"message"`
	NextSteps []string `json:"next_steps,omitempty"`
}
