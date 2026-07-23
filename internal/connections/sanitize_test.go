package connections

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestSanitizeDisplayTextStripsControlsAndEscapes(t *testing.T) {
	c := qt.New(t)

	cases := map[string]struct {
		in   string
		want string
	}{
		"plain text unchanged": {
			in:   "psql",
			want: "psql",
		},
		"CSI color sequence": {
			in:   "evil\x1b[31mred\x1b[0m",
			want: "evil[31mred[0m",
		},
		"OSC-8 hyperlink": {
			in:   "\x1b]8;;https://evil.example\x07label\x1b]8;;\x07",
			want: "]8;;https://evil.examplelabel]8;;",
		},
		"OSC-52 clipboard": {
			in:   "\x1b]52;c;YWJj\x07",
			want: "]52;c;YWJj",
		},
		"BEL and CR overwrite": {
			in:   "hide\rshown\x07",
			want: "hideshown",
		},
		"embedded newline field spoof": {
			in:   "app\npid:             1",
			want: "apppid:             1",
		},
		"C1 CSI single-byte": {
			in:   "x\u009b31my",
			want: "x31my",
		},
		"DEL": {
			in:   "a\x7fb",
			want: "ab",
		},
		"empty": {
			in:   "",
			want: "",
		},
	}

	for name, tc := range cases {
		c.Run(name, func(c *qt.C) {
			c.Assert(SanitizeDisplayText(tc.in), qt.Equals, tc.want)
		})
	}
}

func TestSanitizeMultilineDisplayTextPreservesNewlines(t *testing.T) {
	c := qt.New(t)

	in := "select 1\x1b[31m\nfrom t\r\nwhere id = 1"
	want := "select 1[31m\nfrom t\nwhere id = 1"
	c.Assert(SanitizeMultilineDisplayText(in), qt.Equals, want)
	c.Assert(SanitizeMultilineDisplayText("one line\x1b[0m"), qt.Equals, "one line[0m")
	c.Assert(SanitizeMultilineDisplayText(""), qt.Equals, "")
}
