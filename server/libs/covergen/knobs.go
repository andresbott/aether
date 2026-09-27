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

// GrainKnobName is the well-known knob every style declares for post-process
// film grain. The render pipeline reads it (not the Draw funcs) to layer
// monochrome noise after downsampling, so grain is tuned uniformly across styles.
const GrainKnobName = "grain"

// GrainKnob returns the standard film-grain knob with the given default amount
// (0 disables grain; higher is coarser noise). Every style includes it in its
// Knobs so grain is tunable everywhere and applied consistently by the render
// pipeline. The default preserves each style's shipped grain, so output at the
// defaults is byte-identical.
func GrainKnob(def float64) Knob {
	return Knob{Name: GrainKnobName, Label: "Grain", Min: 0, Max: 12, Step: 1, Default: def}
}

// The well-known text-overlay knob names every TextDrawer style declares (via
// TextOverlayKnobs) so the lab can tune its title overlay live. DrawTextOverlay is
// the sole reader. All carry a "text" token so the knob-wiring guard routes them
// through the text path and the lab groups them under Text.
const (
	TextScaleKnobName            = "text.scale"            // title size, a multiplier on the style's base fraction
	TextSizeSpreadKnobName       = "text.sizeSpread"       // per-seed title-size jitter (0 = uniform)
	TextOpacityKnobName          = "text.opacity"          // title opacity centre
	TextOpacitySpreadKnobName    = "text.opacitySpread"    // per-seed opacity jitter (0 = uniform)
	TextRoamKnobName             = "text.roam"             // placement breadth: 0 pins the style's anchor, 1 auto-places over the calmest spot
	TextTintKnobName             = "text.tint"             // how often the ink takes a palette-derived colour vs auto black/white
	TextSaturationKnobName       = "text.saturation"       // tint-colour saturation centre
	TextSaturationSpreadKnobName = "text.saturationSpread" // per-seed tint-saturation jitter (0 = uniform)
)

// TextOverlayKnobs returns the shared title-overlay knobs every TextDrawer style
// adds to its Knobs (like GrainKnob), in lab display order: each size/opacity
// centre is immediately followed by its per-seed spread, then placement, tint, and
// the tint's saturation (centre + spread). DrawTextOverlay reads them. The defaults
// are the shared house style — larger,
// slightly translucent titles that auto-place and occasionally take a palette
// colour; a style tunes them for itself in the lab. They are read only on the
// text path, so textless output stays byte-identical regardless of their values.
func TextOverlayKnobs() []Knob {
	return []Knob{
		{Name: TextScaleKnobName, Label: "Text size", Min: 0.3, Max: 2, Step: 0.05, Default: 1.55},
		{Name: TextSizeSpreadKnobName, Label: "Text size spread", Min: 0, Max: 3, Step: 0.05, Default: 0.8},
		{Name: TextOpacityKnobName, Label: "Text opacity", Min: 0, Max: 1, Step: 0.05, Default: 0.85},
		{Name: TextOpacitySpreadKnobName, Label: "Text opacity spread", Min: 0, Max: 1, Step: 0.05, Default: 0.75},
		{Name: TextRoamKnobName, Label: "Text placement", Min: 0, Max: 1, Step: 0.05, Default: 1},
		{Name: TextTintKnobName, Label: "Text tint", Min: 0, Max: 1, Step: 0.05, Default: 0.3},
		{Name: TextSaturationKnobName, Label: "Text saturation", Min: 0, Max: 1, Step: 0.05, Default: 0.85},
		{Name: TextSaturationSpreadKnobName, Label: "Text saturation spread", Min: 0, Max: 1, Step: 0.05, Default: 0},
	}
}
