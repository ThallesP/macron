package check

import (
	"fmt"
	"image"
	"regexp"
	"strings"

	"github.com/ThallesP/macron/internal/imagecheck"
)

type ControlResult struct {
	Name     string           `json:"name"`
	Score    imagecheck.Score `json:"score"`
	Rejected bool             `json:"rejected"`
}

type Thresholds struct {
	MinSSIM          float64 `json:"minSSIM"`
	MaxColorDistance float64 `json:"maxColorDistance"`
}

type Verdict struct {
	Passed       bool             `json:"passed"`
	Failures     []string         `json:"failures"`
	Score        imagecheck.Score `json:"score"`
	Thresholds   Thresholds       `json:"thresholds"`
	TextRequired []string         `json:"textRequired"`
	TextMissing  []string         `json:"textMissing"`
	Controls     []ControlResult  `json:"controls"`
}

type EvaluateInput struct {
	Reference      image.Image
	Frame          image.Image
	Ignore         []imagecheck.Rect
	Controls       []imagecheck.Control
	TextRequired   []string
	RecognisedText string
}

var spaces = regexp.MustCompile(`\s+`)

func Evaluate(in EvaluateInput) Verdict {
	verdict := Verdict{
		Score:        imagecheck.Compare(in.Reference, in.Frame, in.Ignore),
		Thresholds:   Thresholds{MinSSIM: imagecheck.MinSSIM, MaxColorDistance: imagecheck.MaxColorDistance},
		TextRequired: in.TextRequired,
		TextMissing:  MissingText(in.TextRequired, in.RecognisedText),
		Controls:     []ControlResult{},
		Failures:     []string{},
	}
	if !verdict.Score.Passes() {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf(
			"the frame does not match the reference: SSIM %.3f (needs >= %.2f), colour distance %.3f (needs <= %.2f)",
			verdict.Score.SSIM, imagecheck.MinSSIM, verdict.Score.ColorDistance, imagecheck.MaxColorDistance))
	}
	if len(verdict.TextMissing) > 0 {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf("menu text not found: %s", strings.Join(verdict.TextMissing, ", ")))
	}
	for _, control := range in.Controls {
		score := imagecheck.Compare(in.Reference, control.Image, in.Ignore)
		result := ControlResult{Name: control.Name, Score: score, Rejected: !score.Passes()}
		verdict.Controls = append(verdict.Controls, result)
		if !result.Rejected {
			verdict.Failures = append(verdict.Failures, fmt.Sprintf(
				"negative control %q passed (SSIM %.3f, colour distance %.3f), so the check cannot tell it from the reference",
				control.Name, score.SSIM, score.ColorDistance))
		}
	}
	verdict.Passed = len(verdict.Failures) == 0
	return verdict
}

func MissingText(required []string, recognised string) []string {
	haystack := normaliseText(recognised)
	missing := []string{}
	for _, text := range required {
		if !strings.Contains(haystack, normaliseText(text)) {
			missing = append(missing, text)
		}
	}
	return missing
}

func normaliseText(text string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(strings.ToLower(text), " "))
}
