package imagecheck

import (
	"image"
	"image/color"
	"image/draw"
)

type Control struct {
	Name  string
	Image image.Image
}

func BrokenRenders(reference image.Image) []Control {
	return []Control{
		{Name: "black frame", Image: Black(reference.Bounds())},
		{Name: "colours inverted", Image: transform(reference, invert)},
		{Name: "upside down", Image: transform(reference, flipVertical)},
		{Name: "shifted by a fifth", Image: transform(reference, shiftFifth)},
	}
}

func Black(bounds image.Rectangle) image.Image {
	out := image.NewRGBA(bounds)
	draw.Draw(out, bounds, image.NewUniform(color.RGBA{A: 255}), image.Point{}, draw.Src)
	return out
}

type pixelSource func(src image.Image, x, y int) color.RGBA

func transform(src image.Image, pick pixelSource) image.Image {
	bounds := src.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.SetRGBA(x, y, pick(src, x, y))
		}
	}
	return out
}

func rgba(c color.Color) color.RGBA {
	r, g, b, _ := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255}
}

func invert(src image.Image, x, y int) color.RGBA {
	c := rgba(src.At(x, y))
	return color.RGBA{R: 255 - c.R, G: 255 - c.G, B: 255 - c.B, A: 255}
}

func flipVertical(src image.Image, x, y int) color.RGBA {
	bounds := src.Bounds()
	return rgba(src.At(x, bounds.Max.Y-1-(y-bounds.Min.Y)))
}

func shiftFifth(src image.Image, x, y int) color.RGBA {
	bounds := src.Bounds()
	shifted := x - bounds.Dx()/5
	if shifted < bounds.Min.X {
		return color.RGBA{A: 255}
	}
	return rgba(src.At(shifted, y))
}
