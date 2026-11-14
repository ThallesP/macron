package imagecheck

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

const (
	MinSSIM          = 0.70
	MaxColorDistance = 0.10
	compareWidth     = 320
	compareHeight    = 240
	thumbWidth       = 16
	thumbHeight      = 12
	ssimWindow       = 8
	ssimStride       = 4
	minStructureStd  = 6.0
)

type Score struct {
	SSIM          float64 `json:"ssim"`
	ColorDistance float64 `json:"colorDistance"`
}

func (s Score) Passes() bool {
	return s.SSIM >= MinSSIM && s.ColorDistance <= MaxColorDistance
}

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

func Compare(reference, candidate image.Image, ignore []Rect) Score {
	ref := Resize(reference, compareWidth, compareHeight)
	got := Resize(candidate, compareWidth, compareHeight)
	maskOut(ref, ignore)
	maskOut(got, ignore)
	return Score{
		SSIM:          ssim(luma(ref), luma(got)),
		ColorDistance: colorDistance(Resize(ref, thumbWidth, thumbHeight), Resize(got, thumbWidth, thumbHeight)),
	}
}

func Resize(src image.Image, width, height int) *image.RGBA {
	bounds := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)
	for y := range height {
		y0, y1 := span(y, scaleY, bounds.Min.Y, bounds.Max.Y)
		for x := range width {
			x0, x1 := span(x, scaleX, bounds.Min.X, bounds.Max.X)
			out.SetRGBA(x, y, average(src, x0, y0, x1, y1))
		}
	}
	return out
}

func span(i int, scale float64, min, max int) (int, int) {
	start := min + int(math.Floor(float64(i)*scale))
	end := min + int(math.Ceil(float64(i+1)*scale))
	return start, clamp(end, start+1, max)
}

func clamp(v, lo, hi int) int {
	return int(math.Min(math.Max(float64(v), float64(lo)), float64(hi)))
}

func average(src image.Image, x0, y0, x1, y1 int) color.RGBA {
	var r, g, b, n uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cr, cg, cb, _ := src.At(x, y).RGBA()
			r += uint64(cr >> 8)
			g += uint64(cg >> 8)
			b += uint64(cb >> 8)
			n++
		}
	}
	return color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 255}
}

func maskOut(img *image.RGBA, ignore []Rect) {
	bounds := img.Bounds()
	for _, r := range ignore {
		area := image.Rect(
			int(r.X*float64(bounds.Dx())),
			int(r.Y*float64(bounds.Dy())),
			int(math.Ceil((r.X+r.Width)*float64(bounds.Dx()))),
			int(math.Ceil((r.Y+r.Height)*float64(bounds.Dy()))),
		)
		draw.Draw(img, area, image.NewUniform(color.RGBA{R: 128, G: 128, B: 128, A: 255}), image.Point{}, draw.Src)
	}
}

func luma(img *image.RGBA) [][]float64 {
	bounds := img.Bounds()
	out := make([][]float64, bounds.Dy())
	for y := range out {
		out[y] = make([]float64, bounds.Dx())
		for x := range out[y] {
			c := img.RGBAAt(x, y)
			out[y][x] = 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
		}
	}
	return out
}

func ssim(a, b [][]float64) float64 {
	const c1 = (0.01 * 255) * (0.01 * 255)
	const c2 = (0.03 * 255) * (0.03 * 255)
	const minVariance = minStructureStd * minStructureStd
	var total float64
	var windows int
	for y := 0; y+ssimWindow <= len(a); y += ssimStride {
		for x := 0; x+ssimWindow <= len(a[0]); x += ssimStride {
			meanA, meanB, varA, varB, cov := windowStats(a, b, x, y)
			if varA < minVariance && varB < minVariance {
				continue
			}
			total += ((2*meanA*meanB + c1) * (2*cov + c2)) / ((meanA*meanA + meanB*meanB + c1) * (varA + varB + c2))
			windows++
		}
	}
	if windows == 0 {
		return 0
	}
	return total / float64(windows)
}

func windowStats(a, b [][]float64, x0, y0 int) (meanA, meanB, varA, varB, cov float64) {
	const n = ssimWindow * ssimWindow
	for y := y0; y < y0+ssimWindow; y++ {
		for x := x0; x < x0+ssimWindow; x++ {
			meanA += a[y][x]
			meanB += b[y][x]
		}
	}
	meanA /= n
	meanB /= n
	for y := y0; y < y0+ssimWindow; y++ {
		for x := x0; x < x0+ssimWindow; x++ {
			da, db := a[y][x]-meanA, b[y][x]-meanB
			varA += da * da
			varB += db * db
			cov += da * db
		}
	}
	return meanA, meanB, varA / (n - 1), varB / (n - 1), cov / (n - 1)
}

func colorDistance(a, b *image.RGBA) float64 {
	var total float64
	bounds := a.Bounds()
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			ca, cb := a.RGBAAt(x, y), b.RGBAAt(x, y)
			total += math.Abs(float64(ca.R)-float64(cb.R)) + math.Abs(float64(ca.G)-float64(cb.G)) + math.Abs(float64(ca.B)-float64(cb.B))
		}
	}
	return total / float64(bounds.Dx()*bounds.Dy()*3*255)
}
