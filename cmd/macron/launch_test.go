package main

import (
	"bytes"
	"regexp"
	"testing"
	"time"
)

func TestGameLogLinesCarryTheTimeAndTheSecondsSinceStart(t *testing.T) {
	var log bytes.Buffer
	progress := timedProgressTo(&log, time.Now().Add(-1500*time.Millisecond))

	progress("prefix ready")
	failTo(progress, errFake{})

	line := regexp.MustCompile(`(?m)^macron: \d\d:\d\d:\d\d\.\d{3}Z \+1\.5\ds prefix ready$`)
	if !line.MatchString(log.String()) {
		t.Errorf("log = %q, want a line with the UTC time and +1.5Xs before the message", log.String())
	}
	failure := regexp.MustCompile(`(?m)^macron: \d\d:\d\d:\d\d\.\d{3}Z \+1\.5\ds UNEXPECTED: broken$`)
	if !failure.MatchString(log.String()) {
		t.Errorf("log = %q, want the failure line timed too", log.String())
	}
}

type errFake struct{}

func (errFake) Error() string { return "broken" }
