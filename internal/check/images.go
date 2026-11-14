package check

import (
	"image"
	"image/draw"
	"image/png"
	"os"
)

func ReadPNG(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return png.Decode(file)
}

func WritePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func Crop(src image.Image, area image.Rectangle) image.Image {
	out := image.NewRGBA(image.Rect(0, 0, area.Dx(), area.Dy()))
	draw.Draw(out, out.Bounds(), src, area.Min, draw.Src)
	return out
}

func ContentArea(window image.Rectangle, windowWidthPoints float64, contentWidth, contentHeight int) image.Rectangle {
	scale := float64(window.Dx()) / windowWidthPoints
	height := min(window.Dy(), int(float64(contentHeight)*scale+0.5))
	width := min(window.Dx(), int(float64(contentWidth)*scale+0.5))
	left := window.Min.X + (window.Dx()-width)/2
	return image.Rect(left, window.Max.Y-height, left+width, window.Max.Y)
}
