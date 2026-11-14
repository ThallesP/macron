package check

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/imagecheck"
)

func picture(seed int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := range 480 {
		for x := range 640 {
			v := uint8((x/(20+seed) + y/(30+seed)) % 2 * 200)
			img.SetRGBA(x, y, color.RGBA{R: v, G: uint8(x / 3), B: uint8(y / 2), A: 255})
		}
	}
	return img
}

func TestEvaluatePassesTheSameFrameWithItsText(t *testing.T) {
	reference := picture(0)

	verdict := Evaluate(EvaluateInput{
		Reference:      reference,
		Frame:          reference,
		Controls:       imagecheck.BrokenRenders(reference),
		TextRequired:   []string{"New Game", "Quit"},
		RecognisedText: "NEW  GAME\nOptions\nquit",
	})

	if !verdict.Passed {
		t.Fatalf("failed: %s", strings.Join(verdict.Failures, "; "))
	}
	if len(verdict.Controls) != 4 {
		t.Fatalf("ran %d controls", len(verdict.Controls))
	}
}

func TestEvaluateFailsOnMissingText(t *testing.T) {
	reference := picture(0)

	verdict := Evaluate(EvaluateInput{Reference: reference, Frame: reference, TextRequired: []string{"Quit"}, RecognisedText: "Options"})

	if verdict.Passed || verdict.TextMissing[0] != "Quit" {
		t.Fatalf("verdict %+v", verdict)
	}
}

func TestEvaluateFailsWhenAControlPasses(t *testing.T) {
	reference := picture(0)

	verdict := Evaluate(EvaluateInput{
		Reference: reference,
		Frame:     reference,
		Controls:  []imagecheck.Control{{Name: "a copy of the reference", Image: reference}},
	})

	if verdict.Passed || !strings.Contains(verdict.Failures[0], "a copy of the reference") {
		t.Fatalf("verdict %+v", verdict)
	}
}

func TestEvaluateFailsOnADifferentFrame(t *testing.T) {
	verdict := Evaluate(EvaluateInput{Reference: picture(0), Frame: picture(17)})

	if verdict.Passed {
		t.Fatalf("a different picture passed: %+v", verdict.Score)
	}
}
