package connections

import "strings"

// SanitizeDisplayText strips ASCII C0 controls, DEL, and C1 controls from text
// that will be written to a terminal. Connection observability fields such as
// query text, application_name, username, and client_addr can contain
// attacker-controlled escape sequences (CSI/OSC) from anyone with connect
// access to the observed branch.
func SanitizeDisplayText(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, r == 0x7f:
			return -1
		case r >= 0x80 && r <= 0x9f:
			return -1
		default:
			return r
		}
	}, value)
}

// SanitizeMultilineDisplayText sanitizes each line of value while preserving
// newline separators so multi-line query text stays readable in human output.
func SanitizeMultilineDisplayText(value string) string {
	if value == "" || !strings.Contains(value, "\n") {
		return SanitizeDisplayText(value)
	}
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		lines[i] = SanitizeDisplayText(line)
	}
	return strings.Join(lines, "\n")
}
