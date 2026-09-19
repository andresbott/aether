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

func TestTextOverlayKnobs(t *testing.T) {
	want := []string{
		TextScaleKnobName, TextSizeSpreadKnobName, TextOpacityKnobName,
		TextOpacitySpreadKnobName, TextRoamKnobName, TextTintKnobName,
		TextSaturationKnobName, TextSaturationSpreadKnobName,
	}
	knobs := TextOverlayKnobs()
	if len(knobs) != len(want) {
		t.Fatalf("TextOverlayKnobs() returned %d knobs, want %d", len(knobs), len(want))
	}
	for i, name := range want {
		k := knobs[i]
		if k.Name != name {
			t.Errorf("knob %d = %q, want %q", i, k.Name, name)
		}
		if !(k.Min <= k.Default && k.Default <= k.Max) {
			t.Errorf("%s default %v outside [%v, %v]", k.Name, k.Default, k.Min, k.Max)
		}
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
