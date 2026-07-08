package printer

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintJSONDoesNotEscapeHTMLPlaceholders(t *testing.T) {
	format := JSON
	var out bytes.Buffer
	p := NewPrinter(&format)
	p.SetResourceOutput(&out)

	if err := p.PrintJSON(map[string]string{"next_step": "pscale branch list <database> --org <org> --format json"}); err != nil {
		t.Fatalf("print json: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "<database>") || !strings.Contains(got, "<org>") {
		t.Fatalf("expected unescaped placeholders, got %s", got)
	}
	if strings.Contains(got, `\u003c`) || strings.Contains(got, `\u003e`) {
		t.Fatalf("expected no escaped angle brackets, got %s", got)
	}
}
