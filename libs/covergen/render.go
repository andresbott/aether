package covergen

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// minVariation is the lowest acceptable per-channel colour spread (see variation)
// for a rendered cover: below it the image is a near-single flat colour. It is
// one of the two quality floors combined by coverScore, and sits below every
// style's natural minimum and above the degenerate tail some styles produce for
// unlucky seeds.
const minVariation = 8.0

// maxVariationAttempts bounds the reseed loop: one initial render plus up to
// maxVariationAttempts-1 reseeds. A cover failing the floors is uncommon per
// seed, so this is effectively never exhausted, and the loop always returns the
// best render seen.
const maxVariationAttempts = 8

// renderStyle renders a deterministic cover for h in style s and encodes it as
// PNG. It goes through renderBest, so a cover that fails the quality floors (see
// coverScore) is rejected and deterministically reseeded.
func renderStyle(h [32]byte, s Style, size int, ks KnobSet) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("covergen: size must be > 0, got %d", size)
	}
	img := renderBest(h, s, size, ks, coverScore, maxVariationAttempts)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("covergen: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// renderBest renders h and, if score reports the result is not good enough
// (score < 1), reseeds the hash deterministically and re-renders — up to
// maxAttempts renders total — returning the highest-scoring image seen. A render
// that scores >= 1 is returned immediately, so well-behaved seeds are
// byte-identical to a single render and only degenerate ones pay for the retries.
func renderBest(h [32]byte, s Style, size int, ks KnobSet, score func(*image.RGBA) float64, maxAttempts int) *image.RGBA {
	best := renderOnce(h, s, size, ks)
	bestScore := score(best)
	for attempt := 1; bestScore < 1 && attempt < maxAttempts; attempt++ {
		h = perturb(h)
		cand := renderOnce(h, s, size, ks)
		if sc := score(cand); sc > bestScore {
			best, bestScore = cand, sc
		}
	}
	return best
}

// perturb derives the next seed hash by re-hashing h, giving a deterministic
// chain of reseeds (same starting h always yields the same sequence). It is
// independent of style selection, which keys off a different hash byte (see
// Generator.StyleFor), so reseeding never changes which style renders.
func perturb(h [32]byte) [32]byte {
	return sha256.Sum256(h[:])
}

// renderOnce draws s at double resolution, downsamples 2x for anti-aliasing, and
// applies grain, returning the final image before PNG encoding:
// rngFromHash -> Draw -> downsample2x -> grain, dispatching through a Style value.
// The grain amount is the resolved "grain" knob (see GrainKnob), so it is tuned
// uniformly across styles here rather than in each Draw func.
func renderOnce(h [32]byte, s Style, size int, ks KnobSet) *image.RGBA {
	rng := rngFromHash(h)
	big := image.NewRGBA(image.Rect(0, 0, size*2, size*2))
	s.Draw(big, rng, ks)
	img := downsample2x(big)
	if g := int(ks.Float(GrainKnobName)); g > 0 {
		addGrain(img, rng, g)
	}
	return img
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

// variation measures how far an image is from a single flat colour: the mean
// of the three per-channel standard deviations (in 0..255 units). It is ~0 for
// a solid fill and grows with contrast, so the render pipeline can reject
// degenerate near-single-colour output.
func variation(img *image.RGBA) float64 {
	b := img.Bounds()
	n := float64(b.Dx() * b.Dy())
	if n == 0 {
		return 0
	}
	var sr, sg, sb, sr2, sg2, sb2 float64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := img.RGBAAt(x, y)
			r, g, bl := float64(p.R), float64(p.G), float64(p.B)
			sr += r
			sg += g
			sb += bl
			sr2 += r * r
			sg2 += g * g
			sb2 += bl * bl
		}
	}
	std := func(sum, sumsq float64) float64 {
		v := sumsq/n - (sum/n)*(sum/n)
		if v < 0 {
			v = 0 // guard tiny negative from float rounding
		}
		return math.Sqrt(v)
	}
	return (std(sr, sr2) + std(sg, sg2) + std(sb, sb2)) / 3
}

// minEdgeProminence is the lowest acceptable edge prominence (see edgeProminence)
// for a rendered cover. Below it the image has no visible shapes — a smooth
// gradient or flat fill — even when its colours span a wide range. Calibrated by
// eye on remix (a clearly visible shape lands around 0.005) and set below the
// natural minimum of the edge-rich styles (rings ~0.025, waves ~0.010).
const minEdgeProminence = 0.005

// edgeStrong is the per-pixel edge magnitude (max abs per-channel difference to
// the right/down neighbour) that counts as a real edge. It sits well above every
// style's film grain (max amount 5 -> ~10 neighbour diff), so grain never counts.
const edgeStrong = 20

// edgeProminence is the fraction of pixels whose edge magnitude exceeds
// edgeStrong — how much of the image is strong, grain-beating edge. A flat fill
// or smooth gradient scores ~0 no matter how wide its colour span; visible shapes
// push it up. It captures "is there a visible feature?", which colour spread
// (variation) cannot: a two-tone gradient has high variation but no edges.
func edgeProminence(img *image.RGBA) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 2 || h < 2 {
		return 0
	}
	absDiff := func(a, c uint8) int {
		if a > c {
			return int(a - c)
		}
		return int(c - a)
	}
	var over int
	for y := b.Min.Y; y < b.Max.Y-1; y++ {
		for x := b.Min.X; x < b.Max.X-1; x++ {
			p := img.RGBAAt(x, y)
			r := img.RGBAAt(x+1, y)
			d := img.RGBAAt(x, y+1)
			e := max(absDiff(p.R, r.R), absDiff(p.G, r.G), absDiff(p.B, r.B),
				absDiff(p.R, d.R), absDiff(p.G, d.G), absDiff(p.B, d.B))
			if e > edgeStrong {
				over++
			}
		}
	}
	return float64(over) / float64((w-1)*(h-1))
}

// coverScore rates a cover against both quality floors — colour spread
// (variation) and edge prominence — each as a ratio to its floor, returning the
// smaller. A score >= 1 clears both; a smaller score is how close the weaker axis
// came, so renderBest can keep the closest candidate when no reseed fully clears.
func coverScore(img *image.RGBA) float64 {
	return math.Min(variation(img)/minVariation, edgeProminence(img)/minEdgeProminence)
}

// addGrain layers subtle monochrome noise so flat fields feel like print.
func addGrain(img *image.RGBA, rng *rand.Rand, amt int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			n := rng.IntN(2*amt+1) - amt
			p := img.RGBAAt(x, y)
			img.SetRGBA(x, y, color.RGBA{paint.ClampU8(int(p.R) + n), paint.ClampU8(int(p.G) + n), paint.ClampU8(int(p.B) + n), 255})
		}
	}
}
