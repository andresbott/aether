package libraries

import (
	"strings"
	"testing"
)

func TestValidatePathRequiresAbsoluteDir(t *testing.T) {
	if _, err := ValidatePath(""); err == nil {
		t.Fatal("expected error for empty path")
	}
	dir := t.TempDir()
	abs, err := ValidatePath(dir)
	if err != nil {
		t.Fatalf("expected valid dir, got %v", err)
	}
	if !strings.HasPrefix(abs, "/") {
		t.Fatalf("expected absolute path, got %q", abs)
	}
}

func TestValidateExcludePatternsRejectsBadRegex(t *testing.T) {
	if err := ValidateExcludePatterns([]string{"["}); err == nil {
		t.Fatal("expected error for bad regex")
	}
	if err := ValidateExcludePatterns([]string{`^\..`}); err != nil {
		t.Fatalf("expected valid regex to pass, got %v", err)
	}
}

func TestValidateName(t *testing.T) {
	if err := ValidateName(""); err == nil {
		t.Fatal("empty name should fail")
	}
	if err := ValidateName("   "); err == nil {
		t.Fatal("whitespace-only name should fail")
	}
	if err := ValidateName(strings.Repeat("a", 201)); err == nil {
		t.Fatal("201-char name should fail (max 200)")
	}
	if err := ValidateName("Main"); err != nil {
		t.Fatalf("simple name should pass, got %v", err)
	}
}

func TestValidateDefaultView(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", true}, // empty coerced to default by caller
		{"albums", true},
		{"artists", true},
		{"songs", false},
		{"Albums", false}, // case-sensitive
		{"  ", false},
	}
	for _, c := range cases {
		err := ValidateDefaultView(c.in)
		gotOK := err == nil
		if gotOK != c.ok {
			t.Errorf("%q: expected ok=%v, got err=%v", c.in, c.ok, err)
		}
	}
}

func TestValidateIcon(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", true}, // empty coerced to default by caller
		{"folder", true},
		{"folder-open", true},
		{"th-large", true},
		{"Folder", false}, // case-sensitive
		{"folder!", false},
		{"folder open", false},
		{"-folder", false},
		{"folder-", false},
		{strings.Repeat("a", 101), false},
	}
	for _, c := range cases {
		err := ValidateIcon(c.in)
		gotOK := err == nil
		if gotOK != c.ok {
			t.Errorf("%q: expected ok=%v, got err=%v", c.in, c.ok, err)
		}
	}
}
