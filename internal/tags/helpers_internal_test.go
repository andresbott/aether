package tags

import "testing"

func TestFirst(t *testing.T) {
	m := map[string][]string{
		"TITLE": {"Hello"},
		"EMPTY": {""},
		"MULTI": {"a", "b"},
	}
	if got := first(m, "title", "TITLE"); got != "Hello" {
		t.Errorf("first found: got %q", got)
	}
	if got := first(m, "missing"); got != "" {
		t.Errorf("first missing: got %q", got)
	}
	// First key present but empty -> falls through to the next key.
	if got := first(m, "EMPTY", "TITLE"); got != "Hello" {
		t.Errorf("first skips empty value: got %q", got)
	}
}

func TestValues(t *testing.T) {
	m := map[string][]string{"ARTIST": {"A", "B"}}
	if got := values(m, "artist", "ARTIST"); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("values found: got %v", got)
	}
	if got := values(m, "missing"); got != nil {
		t.Errorf("values missing: got %v", got)
	}
}

func TestParseInt(t *testing.T) {
	cases := map[string]int{"5": 5, "3/12": 3, "": 0, "abc": 0, " 7 ": 7}
	for in, want := range cases {
		if got := parseInt(in); got != want {
			t.Errorf("parseInt(%q)=%d want %d", in, got, want)
		}
	}
}

func TestParseBool(t *testing.T) {
	if !parseBool("1") || !parseBool("true") {
		t.Error("expected true for 1/true")
	}
	if parseBool("") || parseBool("0") {
		t.Error("expected false for empty/0")
	}
}

func TestParseYear(t *testing.T) {
	cases := []struct {
		tags map[string][]string
		want int
	}{
		{map[string][]string{"DATE": {"2001-05-04"}}, 2001},
		{map[string][]string{"year": {"1999"}}, 1999},
		{map[string][]string{"ORIGINALDATE": {"1985"}}, 1985},
		{map[string][]string{}, 0},
		{map[string][]string{"DATE": {"19"}}, 0},     // too short
		{map[string][]string{"DATE": {"abcd"}}, 0},   // not numeric
		{map[string][]string{"DATE": {"0000"}}, 0},   // not > 0
	}
	for _, c := range cases {
		if got := parseYear(c.tags); got != c.want {
			t.Errorf("parseYear(%v)=%d want %d", c.tags, got, c.want)
		}
	}
}

func TestParseDBPtr(t *testing.T) {
	if got := parseDBPtr("-6.50 dB"); got == nil || *got != -6.5 {
		t.Errorf("parseDBPtr dB suffix: %v", got)
	}
	if got := parseDBPtr("3db"); got == nil || *got != 3 {
		t.Errorf("parseDBPtr db suffix: %v", got)
	}
	if got := parseDBPtr(""); got != nil {
		t.Errorf("parseDBPtr empty: want nil got %v", got)
	}
	if got := parseDBPtr("xyz"); got != nil {
		t.Errorf("parseDBPtr invalid: want nil got %v", got)
	}
}

func TestParseFloatPtr(t *testing.T) {
	if got := parseFloatPtr("0.98"); got == nil || *got != 0.98 {
		t.Errorf("parseFloatPtr: %v", got)
	}
	if got := parseFloatPtr(""); got != nil {
		t.Errorf("parseFloatPtr empty: want nil got %v", got)
	}
	if got := parseFloatPtr("bad"); got != nil {
		t.Errorf("parseFloatPtr invalid: want nil got %v", got)
	}
}

func TestSplitSemicolon(t *testing.T) {
	if got := splitSemicolon("A;B;C"); len(got) != 3 {
		t.Errorf("splitSemicolon basic: %v", got)
	}
	// Trims whitespace and drops empty segments.
	if got := splitSemicolon("A; B ; "); len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("splitSemicolon trims/skips: %v", got)
	}
	if got := splitSemicolon(""); got != nil {
		t.Errorf("splitSemicolon empty: want nil got %v", got)
	}
}
