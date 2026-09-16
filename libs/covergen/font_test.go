// libs/covergen/font_test.go
package covergen

import "testing"

func TestTextEmpty(t *testing.T) {
	if !(Text{}).Empty() {
		t.Fatal("zero Text should be Empty")
	}
	if (Text{Main: "x"}).Empty() {
		t.Fatal("Text with Main should not be Empty")
	}
	if (Text{Subtitle: "y"}).Empty() {
		t.Fatal("Text with Subtitle should not be Empty")
	}
}

func TestTextKnobsIdentityDefaults(t *testing.T) {
	ks := newKnobSet(TextKnobs(), nil)
	if got := ks.Float(TextScaleKnobName); got != 1 {
		t.Fatalf("text.scale default = %v, want 1", got)
	}
	if got := ks.Float(TextOpacityKnobName); got != 1 {
		t.Fatalf("text.opacity default = %v, want 1", got)
	}
}

func TestFontClassesDistinct(t *testing.T) {
	all := []FontClass{FontFormal, FontClean, FontDisplay, FontMono, FontScript}
	seen := map[FontClass]bool{}
	for _, c := range all {
		if c == "" || seen[c] {
			t.Fatalf("class %q empty or duplicated", c)
		}
		seen[c] = true
	}
}
