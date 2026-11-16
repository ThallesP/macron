package check

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ThallesP/macron/internal/scenario"
)

const (
	UploadsTraceEnv   = "WINEDEBUG=-all,+opengl"
	UploadsControlEnv = "WINE_OPENGL_UNSYNCHRONIZED=0"
	measuringLine     = "RESULT measuring"
	unsynchronizedBit = 0x20
)

var (
	traceCall = regexp.MustCompile(`trace:opengl:(\w+) `)
	mapAccess = regexp.MustCompile(`access (\d+)`)
)

type UploadCounts struct {
	Draws                int `json:"draws"`
	BufferSubData        int `json:"bufferSubData"`
	SynchronizedMaps     int `json:"synchronizedMaps"`
	UnsynchronizedMaps   int `json:"unsynchronizedMaps"`
	UnsynchronizedWrites int `json:"unsynchronizedWrites"`
}

type UploadsRun struct {
	Exe     string        `json:"exe"`
	Control bool          `json:"control"`
	Counts  *UploadCounts `json:"counts"`
	Slow    bool          `json:"slow"`
}

type UploadsVerdict struct {
	Passed   bool         `json:"passed"`
	Failures []string     `json:"failures"`
	Source   string       `json:"source"`
	Runs     []UploadsRun `json:"runs"`
}

// ParseUploads counts the buffer uploads and draws an OpenGL trace shows
// while the probe measures, between its "RESULT measuring" and "RESULT fps"
// lines, so start-up uploads (textures, initial data) are left out.
func ParseUploads(output string) (UploadCounts, error) {
	counts := UploadCounts{}
	start := strings.Index(output, measuringLine)
	if start < 0 {
		return counts, fmt.Errorf("the probe printed no %q line", measuringLine)
	}
	end := strings.Index(output[start:], scenario.FrameRateLine)
	if end < 0 {
		return counts, fmt.Errorf("the probe printed no %q line after %q", scenario.FrameRateLine, measuringLine)
	}
	for _, line := range strings.Split(output[start:start+end], "\n") {
		match := traceCall.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name := match[1]
		switch {
		case strings.HasPrefix(name, "glDraw") || strings.HasPrefix(name, "glMultiDraw"):
			counts.Draws++
		case name == "glBufferSubData" || name == "glBufferSubDataARB" || name == "glNamedBufferSubData":
			counts.BufferSubData++
		case name == "glBufferWriteUnsynchronizedWINE":
			counts.UnsynchronizedWrites++
		case name == "glMapBufferRange" || name == "glMapNamedBufferRange":
			if isUnsynchronizedMap(line) {
				counts.UnsynchronizedMaps++
				continue
			}
			counts.SynchronizedMaps++
		case name == "glMapBuffer" || name == "glMapBufferARB" || name == "glMapNamedBuffer":
			counts.SynchronizedMaps++
		}
	}
	return counts, nil
}

func isUnsynchronizedMap(line string) bool {
	match := mapAccess.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	access, err := strconv.Atoi(match[1])
	return err == nil && access&unsynchronizedBit != 0
}

// waits says whether a run's uploads make the host OpenGL wait for the GPU:
// Apple's OpenGL over Metal submits and waits on every glBufferSubData or
// synchronized map into a buffer that issued draws still use.
func (c UploadCounts) waits() bool {
	return c.BufferSubData > 0 || c.SynchronizedMaps > 0
}

type UploadsInput struct {
	Threshold scenario.Uploads
	Runs      []UploadsRunOutput
}

type UploadsRunOutput struct {
	Exe     string
	Control bool
	Output  string
}

func EvaluateUploads(in UploadsInput) UploadsVerdict {
	verdict := UploadsVerdict{Source: in.Threshold.Source, Failures: []string{}, Runs: []UploadsRun{}}
	controls := 0
	for _, run := range in.Runs {
		result := UploadsRun{Exe: run.Exe, Control: run.Control}
		counts, err := ParseUploads(run.Output)
		if err != nil {
			verdict.Failures = append(verdict.Failures, fmt.Sprintf("%s%s: %s", run.Exe, controlLabel(run.Control), err))
			verdict.Runs = append(verdict.Runs, result)
			continue
		}
		result.Counts = &counts
		result.Slow = counts.waits()
		verdict.Runs = append(verdict.Runs, result)
		if counts.Draws == 0 {
			verdict.Failures = append(verdict.Failures, fmt.Sprintf("%s%s: the trace shows no draws while the probe measured, so it says nothing", run.Exe, controlLabel(run.Control)))
			continue
		}
		if run.Control {
			controls++
			if !result.Slow {
				verdict.Failures = append(verdict.Failures, fmt.Sprintf(
					"negative control: %s with %s showed no glBufferSubData and no synchronized map, so the check cannot tell the old upload path",
					run.Exe, UploadsControlEnv))
			}
			continue
		}
		if result.Slow {
			verdict.Failures = append(verdict.Failures, fmt.Sprintf(
				"%s: %d glBufferSubData and %d synchronized buffer maps in %d draws; each one makes Apple's OpenGL wait for the GPU",
				run.Exe, counts.BufferSubData, counts.SynchronizedMaps, counts.Draws))
		}
		if counts.UnsynchronizedWrites+counts.UnsynchronizedMaps < counts.Draws {
			verdict.Failures = append(verdict.Failures, fmt.Sprintf(
				"%s: %d unsynchronized uploads for %d draws; every DrawPrimitiveUP should upload once",
				run.Exe, counts.UnsynchronizedWrites+counts.UnsynchronizedMaps, counts.Draws))
		}
	}
	if controls == 0 {
		verdict.Failures = append(verdict.Failures, "no negative control ran")
	}
	verdict.Passed = len(verdict.Failures) == 0
	return verdict
}

func controlLabel(control bool) string {
	if control {
		return " (control, " + UploadsControlEnv + ")"
	}
	return ""
}

func (v UploadsVerdict) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "| Uploads run | draws | glBufferSubData | synchronized maps | unsynchronized writes | unsynchronized maps | Needs |\n|---|---|---|---|---|---|---|\n")
	for _, run := range v.Runs {
		needs := "no waiting upload"
		if run.Control {
			needs = "waiting uploads (rejected)"
		}
		name := run.Exe + controlLabel(run.Control)
		if run.Counts == nil {
			fmt.Fprintf(&b, "| %s | none | | | | | %s |\n", name, needs)
			continue
		}
		c := run.Counts
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %s |\n", name, c.Draws, c.BufferSubData, c.SynchronizedMaps, c.UnsynchronizedWrites, c.UnsynchronizedMaps, needs)
	}
	fmt.Fprintf(&b, "\nWhy: %s\n\n", v.Source)
	for _, failure := range v.Failures {
		fmt.Fprintf(&b, "- %s\n", failure)
	}
	return b.String()
}
