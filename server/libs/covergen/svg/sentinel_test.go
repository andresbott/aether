package svg

import (
	"bytes"
	"image/color"
	"testing"
)

func TestSubstituteSentinelsRecolours(t *testing.T) {
	src := []byte(`<rect fill="#FF00FF"/><rect fill="#ffff00"/><rect fill="#00FFFF"/>`)
	out := substituteSentinels(src, [3]color.RGBA{{R: 255}, {G: 255}, {B: 255}})
	if bytes.Contains(bytes.ToUpper(out), []byte("#FF00FF")) {
		t.Errorf("slot-0 sentinel still present: %s", out)
	}
	if !bytes.Contains(out, []byte("#FF0000")) {
		t.Errorf("slot-0 not replaced with #FF0000: %s", out)
	}
	if !bytes.Contains(out, []byte("#00FF00")) {
		t.Errorf("slot-1 (lowercase sentinel) not replaced with #00FF00: %s", out)
	}
	if !bytes.Contains(out, []byte("#0000FF")) {
		t.Errorf("slot-2 not replaced with #0000FF: %s", out)
	}
}

func TestHasSentinel(t *testing.T) {
	if !hasSentinel([]byte(`fill="#00ffff"`)) {
		t.Error("expected hasSentinel true for lowercase cyan sentinel")
	}
	if hasSentinel([]byte(`fill="#123456"`)) {
		t.Error("expected hasSentinel false for a non-sentinel colour")
	}
}
