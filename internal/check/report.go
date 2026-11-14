package check

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Report struct {
	Scenario    string             `json:"scenario"`
	Where       string             `json:"where"`
	Passed      bool               `json:"passed"`
	Stage       string             `json:"stage"`
	Error       string             `json:"error,omitempty"`
	Verdict     *Verdict           `json:"verdict,omitempty"`
	Window      *MacWindow         `json:"window,omitempty"`
	TimingsSecs map[string]float64 `json:"timingsSeconds"`
	Notes       []string           `json:"notes"`
}

func (r Report) Write(dir string) error {
	content, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.json"), content, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.md"), []byte(r.Summary()), 0o644)
}

func (r Report) Summary() string {
	var b strings.Builder
	result := "FAILED"
	if r.Passed {
		result = "PASSED"
	}
	fmt.Fprintf(&b, "## %s: %s\n\n", r.Scenario, result)
	fmt.Fprintf(&b, "Where: %s. Last stage: %s.\n\n", r.Where, r.Stage)
	if r.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n\n", r.Error)
	}
	if r.Verdict != nil {
		v := r.Verdict
		fmt.Fprintf(&b, "| Measure | Value | Needs |\n|---|---|---|\n")
		fmt.Fprintf(&b, "| SSIM | %.3f | >= %.2f |\n", v.Score.SSIM, v.Thresholds.MinSSIM)
		fmt.Fprintf(&b, "| Colour distance | %.3f | <= %.2f |\n", v.Score.ColorDistance, v.Thresholds.MaxColorDistance)
		fmt.Fprintf(&b, "| Menu text | %d of %d found | all |\n\n", len(v.TextRequired)-len(v.TextMissing), len(v.TextRequired))
		fmt.Fprintf(&b, "| Negative control | SSIM | Colour distance | Rejected |\n|---|---|---|---|\n")
		for _, c := range v.Controls {
			fmt.Fprintf(&b, "| %s | %.3f | %.3f | %t |\n", c.Name, c.Score.SSIM, c.Score.ColorDistance, c.Rejected)
		}
		b.WriteString("\n")
		for _, f := range v.Failures {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	if len(r.TimingsSecs) > 0 {
		b.WriteString("\nTimings (s):")
		for _, name := range []string{"prepareGame", "runtimeInstall", "firstWindow", "reached", "total"} {
			if value, ok := r.TimingsSecs[name]; ok {
				fmt.Fprintf(&b, " %s %.0f,", name, value)
			}
		}
		b.WriteString("\n")
	}
	for _, note := range r.Notes {
		fmt.Fprintf(&b, "\n> %s\n", note)
	}
	return b.String()
}
