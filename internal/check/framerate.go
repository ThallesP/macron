package check

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ThallesP/macron/internal/scenario"
)

type FrameRateResult struct {
	FPS     float64 `json:"fps"`
	P50Ms   float64 `json:"p50Ms"`
	P95Ms   float64 `json:"p95Ms"`
	P99Ms   float64 `json:"p99Ms"`
	Frames  int     `json:"frames"`
	Adapter string  `json:"adapter"`
}

type FrameRateVerdict struct {
	Passed             bool             `json:"passed"`
	Failures           []string         `json:"failures"`
	MinFPS             float64          `json:"minFps"`
	MinFPSSource       string           `json:"minFpsSource"`
	MinShareOfBaseline float64          `json:"minShareOfBaseline,omitempty"`
	Baseline           *FrameRateResult `json:"baseline,omitempty"`
	Measured           *FrameRateResult `json:"measured"`
	ThrottleMs         int              `json:"throttleMs"`
	Throttled          *FrameRateResult `json:"throttled"`
	ThrottledRejected  bool             `json:"throttledRejected"`
}

type FrameRateInput struct {
	Threshold       scenario.FrameRate
	MeasuredOutput  string
	BaselineOutput  string
	ThrottleMs      int
	ThrottledOutput string
}

const ThrottleArg = "--throttle-ms"

func ParseFrameRate(output string) (FrameRateResult, error) {
	result := FrameRateResult{}
	found := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if _, adapter, ok := strings.Cut(line, "RESULT adapter "); ok {
			result.Adapter = adapter
		}
		_, fields, ok := strings.Cut(line, scenario.FrameRateLine+" ")
		if !ok {
			continue
		}
		parsed, err := parseFrameRateFields(fields)
		if err != nil {
			return FrameRateResult{}, err
		}
		parsed.Adapter = result.Adapter
		result = parsed
		found = true
	}
	if !found {
		return FrameRateResult{}, fmt.Errorf("the probe printed no %q line", scenario.FrameRateLine)
	}
	return result, nil
}

func parseFrameRateFields(fields string) (FrameRateResult, error) {
	shape := scenario.FrameRateLine + " <frames/s> p50_ms <x> p95_ms <y> p99_ms <z> frames <n>"
	words := strings.Fields(fields)
	if len(words) != 9 || words[1] != "p50_ms" || words[3] != "p95_ms" || words[5] != "p99_ms" || words[7] != "frames" {
		return FrameRateResult{}, fmt.Errorf("the frame-rate line %q is not %q", fields, shape)
	}
	numbers := []float64{}
	for _, word := range []string{words[0], words[2], words[4], words[6]} {
		number, err := strconv.ParseFloat(word, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
			return FrameRateResult{}, fmt.Errorf("%q in the frame-rate line %q is not a number of 0 or more", word, fields)
		}
		numbers = append(numbers, number)
	}
	frames, err := strconv.Atoi(words[8])
	if err != nil || frames < 0 {
		return FrameRateResult{}, fmt.Errorf("frames %q in the frame-rate line %q is not a whole number of 0 or more", words[8], fields)
	}
	return FrameRateResult{FPS: numbers[0], P50Ms: numbers[1], P95Ms: numbers[2], P99Ms: numbers[3], Frames: frames}, nil
}

func MinFPSFor(threshold scenario.FrameRate, baseline *FrameRateResult) float64 {
	if baseline == nil {
		return threshold.MinFPS
	}
	return max(threshold.MinFPS, threshold.MinShareOfBaseline*baseline.FPS)
}

func ThrottleMsFor(minFPS float64) int {
	if minFPS <= 0 {
		return 100
	}
	return int(math.Ceil(2000 / minFPS))
}

func parseBaseline(output string) (*FrameRateResult, error) {
	baseline, err := ParseFrameRate(output)
	if err != nil {
		return nil, err
	}
	if baseline.Frames == 0 {
		return nil, fmt.Errorf("it drew no frames")
	}
	return &baseline, nil
}

func EvaluateFrameRate(in FrameRateInput) FrameRateVerdict {
	verdict := FrameRateVerdict{
		MinFPSSource:       in.Threshold.MinFPSSource,
		MinShareOfBaseline: in.Threshold.MinShareOfBaseline,
		ThrottleMs:         in.ThrottleMs,
		Failures:           []string{},
	}
	if in.Threshold.MinShareOfBaseline > 0 {
		baseline, err := parseBaseline(in.BaselineOutput)
		if err != nil {
			verdict.Failures = append(verdict.Failures, fmt.Sprintf("no frame rate from the baseline (%s): %s", in.Threshold.BaselineExe, err))
		}
		verdict.Baseline = baseline
	}
	verdict.MinFPS = MinFPSFor(in.Threshold, verdict.Baseline)
	measured, err := ParseFrameRate(in.MeasuredOutput)
	if err != nil {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf("no frame rate from the probe: %s", err))
	}
	if err == nil {
		verdict.Measured = &measured
	}
	throttled, err := ParseFrameRate(in.ThrottledOutput)
	if err != nil {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf("no frame rate from the throttled probe (negative control): %s", err))
	}
	if err == nil {
		verdict.Throttled = &throttled
		verdict.ThrottledRejected = !meetsThreshold(throttled, verdict.MinFPS)
	}

	if !in.Threshold.HasThreshold() {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf("no frame-rate threshold is set (frameRate.minFps: %s)", in.Threshold.MinFPSSource))
		verdict.Passed = false
		return verdict
	}
	if verdict.Measured != nil && !meetsThreshold(measured, verdict.MinFPS) {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf("the probe ran at %.1f frames/s, the threshold is %.1f", measured.FPS, verdict.MinFPS))
	}
	if verdict.Throttled != nil && !verdict.ThrottledRejected {
		verdict.Failures = append(verdict.Failures, fmt.Sprintf(
			"negative control: the probe throttled by %d ms a frame ran at %.1f frames/s and passed the threshold of %.1f, so the measure cannot tell a slow run",
			in.ThrottleMs, throttled.FPS, verdict.MinFPS))
	}
	verdict.Passed = len(verdict.Failures) == 0
	return verdict
}

func meetsThreshold(result FrameRateResult, minFPS float64) bool {
	return result.Frames > 0 && minFPS > 0 && result.FPS >= minFPS
}

func (v FrameRateVerdict) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Frame rate run | fps | p50 ms | p95 ms | p99 ms | frames | Needs |\n|---|---|---|---|---|---|---|\n")
	threshold := fmt.Sprintf(">= %.1f", v.MinFPS)
	if v.MinFPS <= 0 {
		threshold = "not set"
	}
	rows := []struct {
		name   string
		result *FrameRateResult
		needs  string
	}{
		{"probe", v.Measured, threshold},
		{fmt.Sprintf("throttled by %d ms (control)", v.ThrottleMs), v.Throttled, "rejected"},
	}
	if v.MinShareOfBaseline > 0 {
		rows = append(rows, struct {
			name   string
			result *FrameRateResult
			needs  string
		}{"baseline", v.Baseline, fmt.Sprintf("probe >= %.2f of it", v.MinShareOfBaseline)})
	}
	for _, row := range rows {
		if row.result == nil {
			fmt.Fprintf(&b, "| %s | none | | | | | %s |\n", row.name, row.needs)
			continue
		}
		fmt.Fprintf(&b, "| %s | %.1f | %.2f | %.2f | %.2f | %d | %s |\n", row.name, row.result.FPS, row.result.P50Ms, row.result.P95Ms, row.result.P99Ms, row.result.Frames, row.needs)
	}
	if v.Measured != nil {
		fmt.Fprintf(&b, "\nAdapter: %s. Threshold source: %s.\n", v.Measured.Adapter, v.MinFPSSource)
	}
	b.WriteString("\n")
	for _, failure := range v.Failures {
		fmt.Fprintf(&b, "- %s\n", failure)
	}
	return b.String()
}
