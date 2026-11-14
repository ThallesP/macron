package imagecheck

import (
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

func menu(width, height int, accent color.RGBA) *image.RGBA {
	return menuAt(width, height, accent, 8, 5)
}

func menuAt(width, height int, accent color.RGBA, leftEighths, items int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{R: uint8(20 + 60*y/height), G: uint8(30 + 40*x/width), B: 90, A: 255})
		}
	}
	left := width * leftEighths / 64
	right := left + 3*width/8
	for i := range items {
		top := height/3 + i*height/10
		for y := top; y < top+height/16; y++ {
			for x := left; x < right; x++ {
				img.SetRGBA(x, y, accent)
			}
		}
		for x := left + 6; x < right-6; x += 7 {
			for y := top + 3; y < top+height/16-3; y++ {
				img.SetRGBA(x, y, color.RGBA{R: 250, G: 250, B: 250, A: 255})
				img.SetRGBA(x+1, y, color.RGBA{R: 250, G: 250, B: 250, A: 255})
			}
		}
	}
	return img
}

func noisy(src *image.RGBA, amount int, seed uint64) *image.RGBA {
	random := rand.New(rand.NewPCG(seed, seed))
	out := image.NewRGBA(src.Bounds())
	for i := range src.Pix {
		if i%4 == 3 {
			out.Pix[i] = 255
			continue
		}
		v := int(src.Pix[i]) + random.IntN(2*amount+1) - amount
		out.Pix[i] = uint8(clamp(v, 0, 255))
	}
	return out
}

func offsetByOne(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			out.SetRGBA(x, y, src.RGBAAt(max(x-1, 0), max(y-1, 0)))
		}
	}
	return out
}

func softened(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	bounds := src.Bounds()
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			var r, g, b, n int
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					c := src.RGBAAt(clamp(x+dx, 0, bounds.Dx()-1), clamp(y+dy, 0, bounds.Dy()-1))
					r, g, b, n = r+int(c.R), g+int(c.G), b+int(c.B), n+1
				}
			}
			out.SetRGBA(x, y, color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: 255})
		}
	}
	return out
}

func TestSameImagePasses(t *testing.T) {
	reference := menu(800, 600, color.RGBA{R: 180, G: 40, B: 40, A: 255})

	score := Compare(reference, reference, nil)

	if score.SSIM < 0.999 || score.ColorDistance > 0.001 || !score.Passes() {
		t.Fatalf("identical images scored %+v", score)
	}
}

func TestSameMenuFromAnotherGPUPasses(t *testing.T) {
	reference := menu(800, 600, color.RGBA{R: 180, G: 40, B: 40, A: 255})
	otherGPU := noisy(softened(offsetByOne(menu(800, 600, color.RGBA{R: 172, G: 46, B: 44, A: 255}))), 12, 1)

	score := Compare(reference, otherGPU, nil)
	t.Logf("other GPU: %+v", score)

	if !score.Passes() {
		t.Fatalf("the same menu with noise, rescaling and slightly different colours failed: %+v", score)
	}
}

func TestNegativeControlsFail(t *testing.T) {
	reference := menu(800, 600, color.RGBA{R: 180, G: 40, B: 40, A: 255})
	controls := append(BrokenRenders(reference), Control{
		Name:  "another menu",
		Image: noisy(menuAt(800, 600, color.RGBA{R: 40, G: 160, B: 60, A: 255}, 2, 3), 4, 2),
	})

	for _, control := range controls {
		score := Compare(reference, control.Image, nil)
		t.Logf("%s: %+v", control.Name, score)
		if score.Passes() {
			t.Errorf("%s passed: %+v", control.Name, score)
		}
	}
}

func TestIgnoredRegionsDoNotCount(t *testing.T) {
	reference := menu(800, 600, color.RGBA{R: 180, G: 40, B: 40, A: 255})
	animated := menu(800, 600, color.RGBA{R: 180, G: 40, B: 40, A: 255})
	for y := range 600 {
		for x := 400; x < 800; x++ {
			animated.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x * y), A: 255})
		}
	}

	whole := Compare(reference, animated, nil)
	masked := Compare(reference, animated, []Rect{{X: 0.5, Y: 0, Width: 0.5, Height: 1}})

	if whole.Passes() {
		t.Fatalf("a frame with half of it changed passed: %+v", whole)
	}
	if !masked.Passes() {
		t.Fatalf("the change inside the ignored region still failed: %+v", masked)
	}
}
