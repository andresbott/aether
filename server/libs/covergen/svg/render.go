// Package svg implements the covergen "svg" style: hand-picked parametrized SVG
// motifs recoloured from the seed palette and composited over a generated
// background. Motifs are authored as valid SVG using sentinel fill colours (see
// sentinel.go) and a normalized viewBox="0 0 100 100" with a transparent
// background.
package svg

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// rasterizeMotif parses SVG src and renders it into a w×h RGBA, preserving the
// motif's transparency. Deterministic: same src+size always yields the same
// pixels.
func rasterizeMotif(src []byte, w, h int) (*image.RGBA, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, float64(w), float64(h))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	scanner := rasterx.NewScannerGV(w, h, img, img.Bounds())
	raster := rasterx.NewDasher(w, h, scanner)
	icon.Draw(raster, 1.0)
	return img, nil
}

// paintGradient fills img with a two-colour gradient whose direction (diagonal,
// vertical, horizontal or radial) is chosen from rng, mirroring the other styles'
// backgrounds.
func paintGradient(img *image.RGBA, rng *rand.Rand, c1, c2 color.RGBA) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	dir := rng.IntN(4)
	rcx := fs * (0.2 + rng.Float64()*0.6)
	rcy := fs * (0.2 + rng.Float64()*0.6)
	rmax := 0.0
	for _, p := range [][2]float64{{0, 0}, {fs, 0}, {0, fs}, {fs, fs}} {
		if d := math.Hypot(p[0]-rcx, p[1]-rcy); d > rmax {
			rmax = d
		}
	}
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			var t float64
			switch dir {
			case 0:
				t = float64(x+y) / (2 * (fs - 1))
			case 1:
				t = float64(y) / (fs - 1)
			case 2:
				t = float64(x) / (fs - 1)
			default:
				t = math.Hypot(float64(x)-rcx, float64(y)-rcy) / rmax
			}
			img.SetRGBA(x, y, paint.LerpRGBA(c1, c2, t))
		}
	}
}
