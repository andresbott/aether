package covergen

import (
	"image"
	"math"
	"math/rand/v2"
)

// wavesKnobs tune the synthwave poster. Each is applied after its per-seed
// random draw (Default 1.0 = shipped look).
var wavesKnobs = []Knob{
	{Name: "waves.amp", Label: "Wave height", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "waves.ampSpread", Label: "Wave height spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "waves.freq", Label: "Wave frequency", Min: 0.2, Max: 3, Step: 0.05, Default: 1},
	{Name: "waves.freqSpread", Label: "Wave frequency spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "waves.layers", Label: "Layer count", Min: 0.5, Max: 2.5, Step: 0.05, Default: 1},
	{Name: "waves.layersSpread", Label: "Layer count spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "waves.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "waves.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "waves.hue", Label: "Hue", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "waves.hueSpread", Label: "Hue spread", Min: 0, Max: 10, Step: 0.05, Default: 0},
	{Name: "waves.center", Label: "Wave center", Min: -0.3, Max: 0.3, Step: 0.02, Default: 0},
	{Name: "waves.centerSpread", Label: "Wave center spread", Min: 0, Max: 0.3, Step: 0.02, Default: 0},
}

// drawWaves paints a synthwave-style poster: a vertical sky ramp glowing at
// the horizon, an optional flat sun disc (sometimes with scanline cuts), and
// opaque wave layers that darken and grow as they approach the viewer.
func drawWaves(img *image.RGBA, rng *rand.Rand, ks knobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	amp := ks.Float("waves.amp")
	ampSpread := ks.Float("waves.ampSpread")
	freq := ks.Float("waves.freq")
	freqSpread := ks.Float("waves.freqSpread")
	layerMul := ks.Float("waves.layers")
	layersSpread := ks.Float("waves.layersSpread")
	satMul := ks.Float("waves.saturation")
	satSpread := ks.Float("waves.saturationSpread")
	hueMul := hueMultiplier(rng, ks.Float("waves.hue"), ks.Float("waves.hueSpread"))
	waveCenter := ks.Float("waves.center")
	waveCenterSpread := ks.Float("waves.centerSpread")

	baseHue := rng.Float64() * 360
	span := (40 + rng.Float64()*80) * hueMul
	if rng.IntN(2) == 0 {
		span = -span
	}

	// Sky: dark at the top, glowing near the horizon.
	skyShift := (30 + rng.Float64()*60) * hueMul
	if rng.IntN(2) == 0 {
		skyShift = -skyShift
	}
	rsky := rng.Float64()
	skyTop := hsl(baseHue+skyShift, clampFloat(satMul*(0.55+rsky*0.25)+(satSpread-1)*0.25*(rsky-0.5), 0, 1), 0.12+rng.Float64()*0.10)
	rglow := rng.Float64()
	glow := hsl(baseHue, clampFloat(satMul*(0.80+rglow*0.15)+(satSpread-1)*0.15*(rglow-0.5), 0, 1), 0.62+rng.Float64()*0.12)
	centerOff := waveCenter
	if waveCenterSpread > 0 {
		centerOff += waveCenterSpread * (rng.Float64() - 0.5)
	}
	horizonY := fs * (0.40 + rng.Float64()*0.12 + centerOff)
	for y := 0; y < sz; y++ {
		t := float64(y) / horizonY
		if t > 1 {
			t = 1
		}
		c := lerpRGBA(skyTop, glow, t)
		for x := 0; x < sz; x++ {
			img.SetRGBA(x, y, c)
		}
	}

	// Sun disc, flat, near the horizon.
	if rng.IntN(10) < 7 {
		sunR := fs * (0.10 + rng.Float64()*0.15)
		sunX := fs * (0.25 + rng.Float64()*0.50)
		sunY := horizonY - sunR*(rng.Float64()*0.8)
		sun := hsl(baseHue+rng.Float64()*40-20, clampFloat(0.85*satMul, 0, 1), 0.80+rng.Float64()*0.12)
		scanlines := rng.IntN(2) == 0
		for y := 0; y < sz; y++ {
			fy := float64(y) - sunY
			if scanlines && fy > 0 {
				// Cut horizontal slits out of the lower half, wider near
				// the bottom.
				p := fy / sunR // 0..1 down the lower half
				if math.Mod(p*6, 1) < p*0.55 {
					continue
				}
			}
			for x := 0; x < sz; x++ {
				fx := float64(x) - sunX
				if fx*fx+fy*fy <= sunR*sunR {
					img.SetRGBA(x, y, sun)
				}
			}
		}
	}

	rl := rng.IntN(3)
	layers := int(layerMul*float64(4+rl) + (layersSpread-1)*float64(rl-1))
	if layers < 2 {
		layers = 2
	}
	for i := 0; i < layers; i++ {
		front := float64(i) / float64(layers-1) // 0 back .. 1 front
		baseY := fs * (0.44 + 0.48*front + centerOff)
		ramp := rng.Float64()
		amp1 := fs * (amp*(0.025+0.075*front+ramp*0.03) + (ampSpread-1)*0.03*(ramp-0.5))
		amp2 := amp1 * (0.2 + rng.Float64()*0.4)
		rfreq := rng.Float64()
		freq1 := freq*(0.8+rfreq*1.8) + (freqSpread-1)*1.8*(rfreq-0.5)
		freq2 := freq1 * (2 + rng.Float64())
		ph1 := rng.Float64() * 2 * math.Pi
		ph2 := rng.Float64() * 2 * math.Pi

		h := baseHue + span*front
		l := 0.60 - 0.46*front // recede bright, advance dark
		rcol := rng.Float64()
		c := hsl(h, clampFloat(satMul*(0.65+rcol*0.25)+(satSpread-1)*0.25*(rcol-0.5), 0, 1), l)

		for x := 0; x < sz; x++ {
			fx := float64(x) / fs
			y0 := baseY +
				amp1*math.Sin(2*math.Pi*freq1*fx+ph1) +
				amp2*math.Sin(2*math.Pi*freq2*fx+ph2)
			yi := int(y0)
			if yi < 0 {
				yi = 0
			}
			for y := yi; y < sz; y++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
