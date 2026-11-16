package check

import (
	"strings"
	"testing"

	"github.com/ThallesP/macron/internal/scenario"
)

const uploadsStartup = "0124:trace:opengl:glBufferSubData target 34962, offset 0, size 64, data 0x1000\nRESULT measuring\n"

func traceOf(lines ...string) string {
	return uploadsStartup + strings.Join(lines, "\n") + "\nRESULT fps 10.00 p50_ms 100.000 p95_ms 100.000 p99_ms 100.000 frames 20\n"
}

var (
	unsynchronizedFrame = traceOf(
		"0124:trace:opengl:glBufferWriteUnsynchronizedWINE target 34962, offset 0, size 96, data 0x2000",
		"0124:trace:opengl:glDrawArrays mode 5, first 0, count 4",
		"0124:trace:opengl:glMapBufferRange target 34962, offset 96, length 96, access 34",
		"0124:trace:opengl:glDrawArrays mode 5, first 4, count 4",
	)
	waitingFrame = traceOf(
		"0124:trace:opengl:glBufferSubData target 34962, offset 0, size 96, data 0x2000",
		"0124:trace:opengl:glDrawArrays mode 5, first 0, count 4",
		"0124:trace:opengl:glMapBufferRange target 34962, offset 96, length 96, access 2",
		"0124:trace:opengl:glDrawArrays mode 5, first 4, count 4",
	)
)

func TestParseUploadsCountsOnlyWhileMeasuring(t *testing.T) {
	counts, err := ParseUploads(waitingFrame)
	if err != nil {
		t.Fatal(err)
	}
	if counts != (UploadCounts{Draws: 2, BufferSubData: 1, SynchronizedMaps: 1}) {
		t.Fatalf("unexpected counts %+v", counts)
	}
}

func TestParseUploadsTellsUnsynchronizedMapsApart(t *testing.T) {
	counts, _ := ParseUploads(unsynchronizedFrame)

	if counts != (UploadCounts{Draws: 2, UnsynchronizedWrites: 1, UnsynchronizedMaps: 1}) {
		t.Fatalf("unexpected counts %+v", counts)
	}
}

func TestEvaluateUploadsPassesWithARejectedControl(t *testing.T) {
	verdict := EvaluateUploads(UploadsInput{
		Threshold: scenario.Uploads{Source: "test"},
		Runs: []UploadsRunOutput{
			{Exe: "probe32.exe", Output: unsynchronizedFrame},
			{Exe: "probe32.exe", Control: true, Output: waitingFrame},
		},
	})

	if !verdict.Passed {
		t.Fatalf("expected a pass, got %v", verdict.Failures)
	}
}

func TestEvaluateUploadsFailsOnWaitingUploads(t *testing.T) {
	verdict := EvaluateUploads(UploadsInput{
		Threshold: scenario.Uploads{Source: "test"},
		Runs: []UploadsRunOutput{
			{Exe: "probe32.exe", Output: waitingFrame},
			{Exe: "probe32.exe", Control: true, Output: waitingFrame},
		},
	})

	if verdict.Passed {
		t.Fatal("waiting uploads passed")
	}
}

func TestEvaluateUploadsFailsWhenTheControlDoesNotWait(t *testing.T) {
	verdict := EvaluateUploads(UploadsInput{
		Threshold: scenario.Uploads{Source: "test"},
		Runs: []UploadsRunOutput{
			{Exe: "probe32.exe", Output: unsynchronizedFrame},
			{Exe: "probe32.exe", Control: true, Output: unsynchronizedFrame},
		},
	})

	if verdict.Passed {
		t.Fatal("a control without waiting uploads passed")
	}
}

func TestEvaluateUploadsFailsWithoutDraws(t *testing.T) {
	verdict := EvaluateUploads(UploadsInput{
		Threshold: scenario.Uploads{Source: "test"},
		Runs: []UploadsRunOutput{
			{Exe: "probe32.exe", Output: traceOf()},
			{Exe: "probe32.exe", Control: true, Output: waitingFrame},
		},
	})

	if verdict.Passed {
		t.Fatal("a trace without draws passed")
	}
}
