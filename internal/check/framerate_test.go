package check

import (
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/scenario"
)

const probeOutput = `RESULT window: OK
RESULT adapter Microsoft Basic Render Driver (vendor 0x1414, device 0x008c)
RESULT d3d9 device: OK (hardware vertex processing)
RESULT config mode mix sprites 400 bits 32 warmup_s 2.0 measure_s 10.0 throttle_ms 0 present yes dynamic_panel yes
RESULT present: OK (60 frames in 412 ms)
RESULT measuring
RESULT fps 143.25 p50_ms 6.871 p95_ms 8.120 p99_ms 9.904 frames 1433
`

var threshold = scenario.FrameRate{MinFPS: 50, MinFPSSource: "a test"}

func throttledOutput(fps string) string {
	return strings.Replace(probeOutput, "RESULT fps 143.25", "RESULT fps "+fps, 1)
}

func TestParseFrameRateReadsTheProbesResultLine(t *testing.T) {
	result, err := ParseFrameRate(probeOutput)
	if err != nil {
		t.Fatal(err)
	}

	want := FrameRateResult{FPS: 143.25, P50Ms: 6.871, P95Ms: 8.12, P99Ms: 9.904, Frames: 1433, Adapter: "Microsoft Basic Render Driver (vendor 0x1414, device 0x008c)"}
	if result != want {
		t.Fatalf("got %+v, want %+v", result, want)
	}
}

func TestParseFrameRateFindsALineGluedToAnotherWritersPartialLine(t *testing.T) {
	output := strings.Replace(probeOutput, "RESULT fps", "[mvk-warn] VK_ERROR_OUT_OF_POOL_MEMORY: VkDescriptorPool exhausRESULT fps", 1)

	result, err := ParseFrameRate(output)

	if err != nil || result.FPS != 143.25 || result.Frames != 1433 {
		t.Fatalf("got %+v, %v", result, err)
	}
}

func TestParseFrameRateRejectsMissingOrBrokenLines(t *testing.T) {
	cases := map[string]string{
		"no line":         strings.Replace(probeOutput, "RESULT fps", "RESULT nothing", 1),
		"present failed":  "RESULT window: OK\nRESULT present: FAILED (hr=0x88760868 after 3 frames)\n",
		"missing field":   strings.Replace(probeOutput, " frames 1433", "", 1),
		"renamed field":   strings.Replace(probeOutput, "p95_ms", "p90_ms", 1),
		"not a number":    strings.Replace(probeOutput, "143.25", "fast", 1),
		"negative":        strings.Replace(probeOutput, "143.25", "-1", 1),
		"infinite":        strings.Replace(probeOutput, "143.25", "inf", 1),
		"fraction frames": strings.Replace(probeOutput, "frames 1433", "frames 14.5", 1),
		"split by stderr": strings.Replace(probeOutput, "p95_ms 8.120", "p95_[mvk-warn] VK_ERROR_OUT_OF_POOL_MEMORY\nms 8.120", 1),
	}
	for name, output := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseFrameRate(output); err == nil {
				t.Fatalf("parsed %q", output)
			}
		})
	}
}

func TestEvaluateFrameRatePassesAFastProbeWithASlowControl(t *testing.T) {
	verdict := EvaluateFrameRate(FrameRateInput{Threshold: threshold, MeasuredOutput: probeOutput, ThrottleMs: 40, ThrottledOutput: throttledOutput("24.1")})

	if !verdict.Passed || !verdict.ThrottledRejected || verdict.Measured.FPS != 143.25 {
		t.Fatalf("got %+v", verdict)
	}
}

func TestEvaluateFrameRateFails(t *testing.T) {
	cases := map[string]FrameRateInput{
		"below the threshold":       {Threshold: threshold, MeasuredOutput: throttledOutput("49.9"), ThrottledOutput: throttledOutput("20")},
		"no result line":            {Threshold: threshold, MeasuredOutput: "RESULT present: FAILED (hr=0x88760868 after 3 frames)\n", ThrottledOutput: throttledOutput("20")},
		"throttled control passes":  {Threshold: threshold, MeasuredOutput: probeOutput, ThrottledOutput: throttledOutput("60")},
		"throttled control missing": {Threshold: threshold, MeasuredOutput: probeOutput, ThrottledOutput: ""},
		"no threshold set":          {Threshold: scenario.FrameRate{MinFPSSource: "not set"}, MeasuredOutput: probeOutput, ThrottledOutput: throttledOutput("20")},
		"no frames":                 {Threshold: threshold, MeasuredOutput: strings.Replace(probeOutput, "frames 1433", "frames 0", 1), ThrottledOutput: throttledOutput("20")},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			verdict := EvaluateFrameRate(in)

			if verdict.Passed || len(verdict.Failures) == 0 {
				t.Fatalf("passed: %+v", verdict)
			}
		})
	}
}

func TestThrottleKeepsTheControlBelowHalfTheThreshold(t *testing.T) {
	for _, minFPS := range []float64{1, 24, 30, 50, 59.9, 60, 144, 1000} {
		throttleMs := ThrottleMsFor(minFPS)

		if fastestFPS := 1000 / float64(throttleMs); fastestFPS > minFPS/2 {
			t.Errorf("threshold %.1f: a %d ms sleep still allows %.1f frames/s", minFPS, throttleMs, fastestFPS)
		}
	}
	if throttleMs := ThrottleMsFor(0); throttleMs <= 0 {
		t.Fatalf("no threshold gives a %d ms throttle", throttleMs)
	}
}

func TestFrameRateSummaryShowsBothRuns(t *testing.T) {
	verdict := EvaluateFrameRate(FrameRateInput{Threshold: threshold, MeasuredOutput: probeOutput, ThrottleMs: 40, ThrottledOutput: throttledOutput("24.1")})

	summary := verdict.Summary()

	for _, want := range []string{"| probe | 143.2 |", "throttled by 40 ms", "| 24.1 |", "Microsoft Basic Render Driver"} {
		if !strings.Contains(summary, want) {
			t.Errorf("the summary has no %q:\n%s", want, summary)
		}
	}
}

var shareOfBaseline = scenario.FrameRate{MinFPSSource: "a test", BaselineExe: "probe64.exe", MinShareOfBaseline: 0.8}

func TestEvaluateFrameRateMeasuresAgainstTheBaseline(t *testing.T) {
	verdict := EvaluateFrameRate(FrameRateInput{
		Threshold:       shareOfBaseline,
		MeasuredOutput:  throttledOutput("100"),
		BaselineOutput:  throttledOutput("120"),
		ThrottleMs:      21,
		ThrottledOutput: throttledOutput("40"),
	})

	if !verdict.Passed || verdict.MinFPS != 96 || verdict.Baseline.FPS != 120 {
		t.Fatalf("got %+v", verdict)
	}
	if summary := verdict.Summary(); !strings.Contains(summary, "| baseline | 120.0 |") {
		t.Errorf("the summary has no baseline row:\n%s", summary)
	}
}

func TestEvaluateFrameRateFailsAgainstTheBaseline(t *testing.T) {
	cases := map[string]FrameRateInput{
		"below the share":          {Threshold: shareOfBaseline, MeasuredOutput: throttledOutput("95"), BaselineOutput: throttledOutput("120"), ThrottledOutput: throttledOutput("40")},
		"no baseline line":         {Threshold: shareOfBaseline, MeasuredOutput: throttledOutput("100"), BaselineOutput: "RESULT window: OK\n", ThrottledOutput: throttledOutput("40")},
		"baseline drew nothing":    {Threshold: shareOfBaseline, MeasuredOutput: throttledOutput("100"), BaselineOutput: strings.Replace(probeOutput, "frames 1433", "frames 0", 1), ThrottledOutput: throttledOutput("40")},
		"absolute floor still met": {Threshold: scenario.FrameRate{MinFPS: 110, MinFPSSource: "a test", BaselineExe: "probe64.exe", MinShareOfBaseline: 0.5}, MeasuredOutput: throttledOutput("100"), BaselineOutput: throttledOutput("120"), ThrottledOutput: throttledOutput("40")},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			verdict := EvaluateFrameRate(in)

			if verdict.Passed || len(verdict.Failures) == 0 {
				t.Fatalf("passed: %+v", verdict)
			}
		})
	}
}

func TestMinFPSIsTheHigherOfTheFloorAndTheShare(t *testing.T) {
	baseline := &FrameRateResult{FPS: 50, Frames: 500}

	if got := MinFPSFor(scenario.FrameRate{MinFPS: 30, MinShareOfBaseline: 0.8}, baseline); got != 40 {
		t.Errorf("share wins: got %v", got)
	}
	if got := MinFPSFor(scenario.FrameRate{MinFPS: 45, MinShareOfBaseline: 0.8}, baseline); got != 45 {
		t.Errorf("floor wins: got %v", got)
	}
	if got := MinFPSFor(scenario.FrameRate{MinFPS: 30, MinShareOfBaseline: 0.8}, nil); got != 30 {
		t.Errorf("no baseline: got %v", got)
	}
}
