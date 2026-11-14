package check

import (
	"image"
	"testing"
)

func TestContentAreaDropsTheTitleBar(t *testing.T) {
	cases := []struct {
		name   string
		window image.Rectangle
		points float64
		want   image.Rectangle
	}{
		{"1x with a 28 point title bar", image.Rect(0, 0, 640, 508), 640, image.Rect(0, 28, 640, 508)},
		{"Retina", image.Rect(0, 0, 1280, 1016), 640, image.Rect(0, 56, 1280, 1016)},
		{"no title bar", image.Rect(0, 0, 640, 480), 640, image.Rect(0, 0, 640, 480)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ContentArea(c.window, c.points, 640, 480); got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
