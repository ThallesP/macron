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
		"0124:trace:fps:wglSwapBuffers @ approx 30.0fps, total 30.0fps",
		"0124:trace:fps:wined3d_cs_exec_present @ approx 30.0fps",
		"0124:trace:fps:wglSwapBuffers @ approx 60.5fps, total 45.0fps",
		"0124:trace:fps:wined3d_cs_exec_present @ approx 60.5fps",
		"0124:trace:fps:wined3d_cs_exec_present @ approx 59.0fps",
	}, "\n")

	log, isFrameRateLog := readFrameRateLog("steam-1/a.log", text)

	if !isFrameRateLog || log.Exe != "Game.exe" || log.Backend != "dxmt" || !log.HasBridge || len(log.Rates) != 3 {
		t.Fatalf("unexpected log: %+v", log)
	}
	if summary := log.summary(); !strings.Contains(summary, "frame counter: 3 samples, median 59.0 fps, lowest 30.0, highest 60.5") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}

func TestReadFrameRateLogFallsBackToOpenGLSwaps(t *testing.T) {
	text := "macron: logging the frame rate (show-fps exists)\n0124:trace:fps:wglSwapBuffers @ approx 42.0fps, total 41.0fps"

	log, _ := readFrameRateLog("steam-1/a.log", text)

	if len(log.Rates) != 1 || log.Rates[0] != 42 {
		t.Fatalf("unexpected rates: %v", log.Rates)
	}
}

func TestReadFrameRateLogCountsMetalHUDHitches(t *testing.T) {
	text := strings.Join([]string{
		"macron: logging the frame rate (show-fps exists)",
		"macron: starting Overwatch.exe with the dxmt backend",
		"2026-11-16 02:00:20.100 wine[123:456] metal-HUD: 100,512.0,1024.0,16.6,9.1,16.8,9.0",
		"2026-11-16 02:00:20.200 wine[123:456] metal-HUD: 102,512.0,1024.0,3,120.5,9.2,60.0,9.0",
	}, "\n")

	log, _ := readFrameRateLog("steam-2357570/a.log", text)

	if len(log.Intervals) != 4 {
		t.Fatalf("intervals %v, want 4", log.Intervals)
	}
	if summary := log.summary(); !strings.Contains(summary, "4 frames, 18.7 fps on average, 2 longer than 50 ms, 1 longer than 100 ms, the longest 120 ms") {
		t.Fatalf("unexpected summary: %s", summary)
	}
}

func TestReadFrameRateLogSkipsStartsWithoutTheCounter(t *testing.T) {
	if _, isFrameRateLog := readFrameRateLog("steam-1/a.log", "macron: starting Game.exe with the dxmt backend"); isFrameRateLog {
		t.Fatal("a start without show-fps was read as a frame-rate log")
	}
}
