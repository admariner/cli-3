package cmdutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// JSONErrorResponse is the fallback JSON envelope for unhandled command errors.
type JSONErrorResponse struct {
	Status    string   `json:"status"`
	Code      string   `json:"code,omitempty"`
	Error     string   `json:"error"`
	NextSteps []string `json:"next_steps"`
}

// GlobalJSONError classifies an error for the global JSON envelope.
//
// Classification is by message text because this is the top-level fallback:
// command-specific handlers with typed errors have already had their chance.
// next_steps must earn their place: each case lists only the commands that
// actually address the failure, not generic bootstrap commands.
func GlobalJSONError(err error) JSONErrorResponse {
	msg := err.Error()
	status := "error"
	code := "COMMAND_FAILED"

	if cmdErr, ok := errors.AsType[*Error](err); ok {
		if cmdErr.Msg != "" {
			msg = cmdErr.Msg
		}
		if cmdErr.ExitCode == ActionRequestedExitCode {
			status = "action_required"
		}
	}

	var nextSteps []string
	lower := strings.ToLower(msg)

	switch {
	// Auth problems: the fix is logging in (or fixing credentials), then verifying.
	case strings.Contains(msg, WarnAuthMessage) ||
		strings.Contains(lower, "not authenticated") ||
		strings.Contains(msg, "access token has expired"):
		status = "action_required"
		code = "NO_AUTH"
		nextSteps = []string{AgentAuthLoginCmd(), AgentAuthCheckCmd()}

	case strings.Contains(msg, "Authentication failed") && strings.Contains(msg, "service token"):
		status = "action_required"
		code = "SERVICE_TOKEN_INVALID"
		nextSteps = []string{
			"Verify --service-token-id and --service-token values (or PLANETSCALE_SERVICE_TOKEN_ID / PLANETSCALE_SERVICE_TOKEN)",
			AgentAuthCheckCmd(),
		}

	// --org on pscale root: show the corrected shape, not a re-auth loop.
	case strings.Contains(msg, "unknown flag: --org"):
		code = "INVALID_FLAG_PLACEMENT"
		nextSteps = []string{
			"Move --org onto the resource subcommand",
			AgentDatabaseListCmd(""),
		}

	// Wrong invocation: cobra's message already names the missing piece and
	// includes usage. Point at the guide for command shapes; auth is not the issue.
	case strings.Contains(msg, "missing argument") ||
		strings.Contains(msg, "missing arguments") ||
		strings.Contains(msg, "missing required flags") ||
		strings.Contains(msg, "required flag"):
		status = "action_required"
		code = "INVALID_USAGE"
		nextSteps = []string{
			"Re-run with the missing arguments or flags named in the error message",
			AgentGuideCmd(),
		}

	case strings.Contains(msg, "unknown command"):
		code = "UNKNOWN_COMMAND"
		nextSteps = []string{
			"Run `pscale --help` to list valid commands",
			AgentGuideCmd(),
		}

	case strings.Contains(msg, "unknown flag") || strings.Contains(msg, "unknown shorthand flag"):
		code = "UNKNOWN_FLAG"
		nextSteps = []string{
			"Run the same command with --help to list valid flags",
			AgentGuideCmd(),
		}

	// TTY-gated commands: JSON mode is the non-interactive path.
	case strings.Contains(msg, "requires an interactive shell"):
		status = "action_required"
		code = "TTY_REQUIRED"
		nextSteps = []string{AgentAuthLoginCmd()}

	// Confirmation-gated commands: the fix is user approval, then --force.
	case strings.Contains(msg, "run with --force"):
		status = "action_required"
		code = "CONFIRMATION_REQUIRED"
		nextSteps = []string{
			"Ask the user to approve this action",
			"Re-run the same command with --force after approval",
		}

	// Resource lookups that failed: discovery commands are the way forward.
	case strings.Contains(msg, "does not exist"):
		code = "NOT_FOUND"
		nextSteps = []string{
			AgentOrgListCmd(),
			AgentDatabaseListCmd(""),
		}

	// Connectivity: nothing an agent can self-serve beyond retry and status.
	case strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "no such host") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "temporary failure"):
		code = "NETWORK_ERROR"
		nextSteps = []string{
			"Check network connectivity and --api-url, then retry",
			AgentAuthCheckCmd(),
		}

	// Unclassified: auth state is the one thing worth ruling out first.
	default:
		nextSteps = []string{
			AgentAuthCheckCmd(),
			AgentGuideCmd(),
		}
	}

	return JSONErrorResponse{
		Status:    status,
		Code:      code,
		Error:     msg,
		NextSteps: nextSteps,
	}
}

// ReportGlobalJSONError writes the classified envelope for err to w and
// returns the process exit code. On encoding failure it falls back to a
// plain error line on errW.
func ReportGlobalJSONError(w, errW io.Writer, err error) int {
	resp := GlobalJSONError(err)
	if writeErr := WriteJSONError(w, resp); writeErr != nil {
		fmt.Fprintf(errW, "Error: %s\n", err)
	}

	if resp.Status == "action_required" {
		return ActionRequestedExitCode
	}
	if cmdErr, ok := errors.AsType[*Error](err); ok && cmdErr.ExitCode != 0 {
		return cmdErr.ExitCode
	}
	return FatalErrExitCode
}

// WriteJSONError writes a JSON error envelope to w.
func WriteJSONError(w io.Writer, resp JSONErrorResponse) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}
