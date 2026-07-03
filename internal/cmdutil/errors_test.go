package cmdutil

import "testing"

func TestJSONReportedErrorHandled(t *testing.T) {
	err := JSONReportedError(ActionRequestedExitCode)
	if !err.Handled {
		t.Fatal("expected Handled")
	}
	if err.Error() != "" {
		t.Fatalf("expected empty error message, got %q", err.Error())
	}
}
