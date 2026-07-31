package covergen

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

// hsl converts HSL to RGBA like hslToRGBA but first normalises h into
// [0, 360), so callers can pass hue arithmetic results directly.
func hsl(h, s, l float64) color.RGBA {
	return hslToRGBA(math.Mod(math.Mod(h, 360)+360, 360), s, l)
}

// vivid returns n saturated colours built from a random harmony scheme
// (complementary, triadic, analogous, split-complementary).
func vivid(rng *rand.Rand, n int) []color.RGBA {
	base := rng.Float64() * 360
	schemes := [][]float64{
		{0, 180, 30, 210, 60},
		{0, 120, 240, 60, 180},
		{0, 30, 60, -30, 90},
		{0, 150, 210, 30, 180},
	}
	offs := schemes[rng.IntN(len(schemes))]
	out := make([]color.RGBA, n)
	for i := range out {
		h := base + offs[i%len(offs)]
		s := 0.65 + rng.Float64()*0.30
		l := 0.40 + rng.Float64()*0.25
		out[i] = hsl(h, s, l)
	}
	return out
}

// downsample2x box-filters src (which must be square with even dimensions)
// down to half resolution, anti-aliasing hard shape edges.
func downsample2x(src *image.RGBA) *image.RGBA {
	half := src.Bounds().Dx() / 2
	dst := image.NewRGBA(image.Rect(0, 0, half, half))
	for y := 0; y < half; y++ {
		for x := 0; x < half; x++ {
			var r, g, b int
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					p := src.RGBAAt(x*2+dx, y*2+dy)
					r += int(p.R)
					g += int(p.G)
					b += int(p.B)
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / 4), uint8(g / 4), uint8(b / 4), 255})
		}
	}
	return dst
}

// addGrain layers subtle monochrome noise so flat fields feel like print.
func addGrain(img *image.RGBA, rng *rand.Rand, amt int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			n := rng.IntN(2*amt+1) - amt
			p := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{clampU8(int(p.R) + n), clampU8(int(p.G) + n), clampU8(int(p.B) + n), 255})
		}
	}
}

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
