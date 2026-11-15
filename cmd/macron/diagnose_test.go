package main

import (
	"strings"
	"testing"
)

func TestScrubberHidesTheUserAndSteamIDs(t *testing.T) {
	scrub := scrubber("/Users/somebody")

	got := scrub("log /Users/somebody/Library/x.log, somebody, [U:1:643578135], id 76561198012345678")

	if strings.Contains(got, "somebody") || strings.Contains(got, "643578135") || strings.Contains(got, "76561198012345678") {
		t.Fatalf("scrubbed text still identifies the user: %q", got)
	}
	if !strings.Contains(got, "~/Library/x.log") {
		t.Fatalf("home folder not shortened: %q", got)
	}
}

func TestReadFrameRateLogSummarisesTheCounter(t *testing.T) {
	text := strings.Join([]string{
		"macron: logging the frame rate (show-fps exists)",
		"macron: starting Game.exe with the dxmt backend",
		"macron: starting it through steam.exe, which connects it to the Mac Steam client (Steam bridge)",
		"0124:trace:fps:wined3d_cs_exec_present @ approx 30.0fps",
		"0124:trace:fps:wined3d_cs_exec_present @ approx 60.5fps",
		"0124:trace:fps:wined3d_cs_exec_present @ approx 59.0fps",
	}, "\n")

	log, isFrameRateLog := readFrameRateLog("steam-1/a.log", text)

	if !isFrameRateLog || log.Exe != "Game.exe" || log.Backend != "dxmt" || !log.HasBridge || len(log.Rates) != 3 {
		t.Fatalf("unexpected log: %+v", log)
	}
	if summary := log.summary(); !strings.Contains(summary, "median 59.0 fps, lowest 30.0, highest 60.5") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}

func TestReadFrameRateLogSkipsStartsWithoutTheCounter(t *testing.T) {
	if _, isFrameRateLog := readFrameRateLog("steam-1/a.log", "macron: starting Game.exe with the dxmt backend"); isFrameRateLog {
		t.Fatal("a start without show-fps was read as a frame-rate log")
	}
}
