package libraries

import (
	"strings"
	"testing"
)

func TestValidatePathRequiresAbsoluteDir(t *testing.T) {
	if _, err := validatePath(""); err == nil {
		t.Fatal("expected error for empty path")
	}
	dir := t.TempDir()
	abs, err := validatePath(dir)
	if err != nil {
		t.Fatalf("expected valid dir, got %v", err)
	}
	if !strings.HasPrefix(abs, "/") {
		t.Fatalf("expected absolute path, got %q", abs)
	}
}

func TestValidateExcludePatternsRejectsBadRegex(t *testing.T) {
	if err := validateExcludePatterns([]string{"["}); err == nil {
		t.Fatal("expected error for bad regex")
	}
	if err := validateExcludePatterns([]string{`^\..`}); err != nil {
		t.Fatalf("expected valid regex to pass, got %v", err)
	}
}

func TestValidateName(t *testing.T) {
	if err := validateName(""); err == nil {
		t.Fatal("empty name should fail")
	}
	if err := validateName("   "); err == nil {
		t.Fatal("whitespace-only name should fail")
	}
	if err := validateName(strings.Repeat("a", 201)); err == nil {
		t.Fatal("201-char name should fail (max 200)")
	}
	if err := validateName("Main"); err != nil {
		t.Fatalf("simple name should pass, got %v", err)
	}
}

func TestValidateDefaultView(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", true},        // empty coerced to default by caller
		{"albums", true},
		{"artists", true},
		{"songs", false},
		{"Albums", false}, // case-sensitive
		{"  ", false},
	}
	for _, c := range cases {
		err := validateDefaultView(c.in)
		gotOK := err == nil
		if gotOK != c.ok {
			t.Errorf("%q: expected ok=%v, got err=%v", c.in, c.ok, err)
		}
	}
}
