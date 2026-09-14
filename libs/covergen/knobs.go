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

// KnobSet holds resolved knob values for one render: the style's declared
// defaults with any overrides applied. Overrides for keys the style does not
// declare are ignored, so a stray lab URL param cannot inject a value that no
// draw func reads.
type KnobSet struct {
	v map[string]float64
}

// newKnobSet resolves knob defaults+overrides for a set of declared knobs,
// starting from their defaults and applying any overrides on top. Overrides for
// keys not in knobs are ignored.
func newKnobSet(knobs []Knob, overrides map[string]float64) KnobSet {
	ks := KnobSet{v: make(map[string]float64)}
	for _, k := range knobs {
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
func (k KnobSet) Float(name string) float64 { return k.v[name] }
