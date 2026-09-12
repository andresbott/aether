package covergen

// Knob describes one tunable parameter of a rendering style, for the covergen
// lab's slider UI. Values are float64 so one generic control serves ints,
// scales, and toggles alike. A knob is a multiplier or offset applied after
// its per-seed random draw, and a Default of the identity value (1.0 for a
// multiplier, 0 for an offset) keeps shipped output byte-identical.
type Knob struct {
	Name    string // stable key read by the style's draw func, e.g. "rings.spacing"
	Label   string // human label for the lab UI
	Min     float64
	Max     float64
	Step    float64
	Default float64
}

// Knobs returns a style's tunable parameters in display order. Styles that are
// not yet parameterised return nil, and their output ignores any overrides.
func (s Style) Knobs() []Knob {
	switch s {
	case StyleClassic:
		return classicKnobs
	case StyleRings:
		return ringsKnobs
	case StyleWaves:
		return wavesKnobs
	case StyleBauhaus:
		return bauhausKnobs
	case StylePoster:
		return posterKnobs
	case StyleRemix:
		return remixKnobs
	default:
		return nil
	}
}

// knobSet holds resolved knob values for one render: the style's declared
// defaults with any overrides applied. Overrides for keys the style does not
// declare are ignored, so a stray lab URL param cannot inject a value that no
// draw func reads.
type knobSet struct {
	v map[string]float64
}

func newKnobSet(style Style, overrides map[string]float64) knobSet {
	ks := knobSet{v: make(map[string]float64)}
	for _, k := range style.Knobs() {
		ks.v[k.Name] = k.Default
	}
	for name, val := range overrides {
		if _, ok := ks.v[name]; ok {
			ks.v[name] = val
		}
	}
	return ks
}

// Float returns the resolved value for name, or 0 if the style has no such knob.
func (k knobSet) Float(name string) float64 { return k.v[name] }
