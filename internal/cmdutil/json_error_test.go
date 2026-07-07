package cmdutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestGlobalJSONErrorAuthRequired(t *testing.T) {
	resp := GlobalJSONError(errors.New("not authenticated yet. Please run 'pscale auth login'"))
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Code != "NO_AUTH" {
		t.Fatalf("code = %q", resp.Code)
	}
	if len(resp.NextSteps) == 0 || resp.NextSteps[0] != AgentAuthLoginCmd() {
		t.Fatalf("next_steps = %#v", resp.NextSteps)
	}
}

func TestGlobalJSONErrorInvalidUsageSkipsAuth(t *testing.T) {
	resp := GlobalJSONError(errors.New("missing argument <database> \n\nUsage: ..."))
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Code != "INVALID_USAGE" {
		t.Fatalf("code = %q", resp.Code)
	}
	for _, step := range resp.NextSteps {
		if step == AgentAuthCheckCmd() || step == AgentAuthLoginCmd() {
			t.Fatalf("invalid usage should not suggest auth commands, got %#v", resp.NextSteps)
		}
	}
}

func TestGlobalJSONErrorOrgFlagPlacement(t *testing.T) {
	resp := GlobalJSONError(errors.New("unknown flag: --org"))
	if resp.Code != "INVALID_FLAG_PLACEMENT" {
		t.Fatalf("code = %q", resp.Code)
	}
	if len(resp.NextSteps) != 2 || resp.NextSteps[1] != AgentDatabaseListCmd("") {
		t.Fatalf("next_steps = %#v", resp.NextSteps)
	}
}

func TestGlobalJSONErrorUnknownFlag(t *testing.T) {
	resp := GlobalJSONError(errors.New("unknown flag: --bogus"))
	if resp.Code != "UNKNOWN_FLAG" {
		t.Fatalf("code = %q", resp.Code)
	}
}

func TestGlobalJSONErrorUnknownCommand(t *testing.T) {
	resp := GlobalJSONError(errors.New(`unknown command "wat" for "pscale"`))
	if resp.Code != "UNKNOWN_COMMAND" {
		t.Fatalf("code = %q", resp.Code)
	}
	for _, step := range resp.NextSteps {
		if step == AgentAuthCheckCmd() {
			t.Fatalf("unknown command should not suggest auth check, got %#v", resp.NextSteps)
		}
	}
}

func TestGlobalJSONErrorConfirmationRequired(t *testing.T) {
	resp := GlobalJSONError(errors.New("cannot confirm deletion of database (run with --force to override)"))
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Code != "CONFIRMATION_REQUIRED" {
		t.Fatalf("code = %q", resp.Code)
	}
	if len(resp.NextSteps) != 2 {
		t.Fatalf("next_steps = %#v", resp.NextSteps)
	}
}

func TestGlobalJSONErrorNotFound(t *testing.T) {
	resp := GlobalJSONError(errors.New("database foo does not exist in organization bar"))
	if resp.Code != "NOT_FOUND" {
		t.Fatalf("code = %q", resp.Code)
	}
	if len(resp.NextSteps) == 0 || resp.NextSteps[0] != AgentOrgListCmd() {
		t.Fatalf("next_steps = %#v", resp.NextSteps)
	}
}

func TestGlobalJSONErrorNetwork(t *testing.T) {
	resp := GlobalJSONError(errors.New("dial tcp: lookup api.planetscale.com: no such host"))
	if resp.Code != "NETWORK_ERROR" {
		t.Fatalf("code = %q", resp.Code)
	}
}

func TestGlobalJSONErrorServiceToken(t *testing.T) {
	resp := GlobalJSONError(errors.New("Authentication failed. Your service token appears to be invalid."))
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Code != "SERVICE_TOKEN_INVALID" {
		t.Fatalf("code = %q", resp.Code)
	}
}

func TestGlobalJSONErrorGenericFallback(t *testing.T) {
	resp := GlobalJSONError(errors.New("something went wrong"))
	if resp.Status != "error" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Code != "COMMAND_FAILED" {
		t.Fatalf("code = %q", resp.Code)
	}
	if len(resp.NextSteps) != 2 || resp.NextSteps[0] != AgentAuthCheckCmd() {
		t.Fatalf("next_steps = %#v", resp.NextSteps)
	}
}

func TestGlobalJSONErrorUsesCmdErrorMessage(t *testing.T) {
	resp := GlobalJSONError(&Error{
		Msg:      "You are not authenticated.",
		ExitCode: ActionRequestedExitCode,
	})
	if resp.Error != "You are not authenticated." {
		t.Fatalf("error = %q", resp.Error)
	}
	if resp.Status != "action_required" {
		t.Fatalf("status = %q", resp.Status)
	}
}

func TestReportGlobalJSONError(t *testing.T) {
	var out, errOut bytes.Buffer
	exit := ReportGlobalJSONError(&out, &errOut, errors.New("boom"))
	if exit != FatalErrExitCode {
		t.Fatalf("exit = %d", exit)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}

	var resp JSONErrorResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Status != "error" {
		t.Fatalf("status = %q", resp.Status)
	}
	if resp.Error == "" || len(resp.NextSteps) == 0 {
		t.Fatalf("resp = %#v", resp)
	}
}

func TestReportGlobalJSONErrorActionRequiredExitCode(t *testing.T) {
	var out, errOut bytes.Buffer
	exit := ReportGlobalJSONError(&out, &errOut, &Error{
		Msg:      "needs action",
		ExitCode: ActionRequestedExitCode,
	})
	if exit != ActionRequestedExitCode {
		t.Fatalf("exit = %d", exit)
	}
}
