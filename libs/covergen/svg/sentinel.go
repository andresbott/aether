package svg

import (
	"fmt"
	"image/color"
	"strings"
)

// sentinels are the documented placeholder fill colours an SVG asset uses; the
// renderer substitutes them with the per-seed palette (slot 0/1/2). Authors may
// write them upper- or lower-case.
var sentinels = [3]string{"#FF00FF", "#FFFF00", "#00FFFF"}

// substituteSentinels replaces every sentinel token in src (case-insensitively)
// with the resolved palette colour as #RRGGBB, returning the recoloured SVG.
func substituteSentinels(src []byte, cols [3]color.RGBA) []byte {
	s := string(src)
	for i, tok := range sentinels {
		hex := fmt.Sprintf("#%02X%02X%02X", cols[i].R, cols[i].G, cols[i].B)
		s = strings.ReplaceAll(s, tok, hex)
		s = strings.ReplaceAll(s, strings.ToLower(tok), hex)
	}
	return []byte(s)
}

// hasSentinel reports whether src uses at least one sentinel colour.
func hasSentinel(src []byte) bool {
	up := strings.ToUpper(string(src))
	for _, tok := range sentinels {
		if strings.Contains(up, tok) {
			return true
		}
	}
	return false
}
