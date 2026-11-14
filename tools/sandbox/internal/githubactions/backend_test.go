package githubactions

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

type fakeGitHub struct {
	responses  map[string]string
	notFoundAt map[string]int
	calls      []string
	bodies     []any
}

func (f *fakeGitHub) api(_ context.Context, method, path string, body any) ([]byte, error) {
	call := method + " " + path
	f.calls = append(f.calls, call)
	f.bodies = append(f.bodies, body)
	if f.notFoundAt[call] > 0 {
		f.notFoundAt[call]--
		return nil, fmt.Errorf("GitHub %s: %w", call, ErrNotFound)
	}
	response, isKnown := f.responses[call]
	if !isKnown {
		return nil, fmt.Errorf("unexpected call %s", call)
	}
	return []byte(response), nil
}

func announcementArchive() string {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, _ := writer.Create("sandbox.json")
	file.Write([]byte(`{"id":"sbx_abcd2345","transport":"cloudflare-quick","user":"runner","hostKey":"ssh-ed25519 AAAA","sealedEndpoint":"c2VhbGVk","autoStopAt":1800000000,"hardStopAt":1800019800}`))
	writer.Close()
	return archive.String()
}

const runsPath = "GET repos/o/r/actions/workflows/sandbox.yml/runs?event=workflow_dispatch&per_page=100"

func runJSON(id int, title, status, conclusion string) string {
	return fmt.Sprintf(`{"id":%d,"display_title":%q,"status":%q,"conclusion":%q,"created_at":"2026-11-13T23:00:00Z","html_url":"https://github.com/o/r/actions/runs/%d"}`, id, title, status, conclusion, id)
}

func TestRunState(t *testing.T) {
	cases := []struct {
		status, conclusion string
		isReady            bool
		want               sandbox.State
	}{
		{"queued", "", false, sandbox.StateStarting},
		{"in_progress", "", false, sandbox.StateStarting},
		{"in_progress", "", true, sandbox.StateReady},
		{"completed", "success", false, sandbox.StateStopped},
		{"completed", "cancelled", false, sandbox.StateStopped},
		{"completed", "timed_out", false, sandbox.StateStopped},
		{"completed", "failure", false, sandbox.StateFailed},
		{"completed", "startup_failure", false, sandbox.StateFailed},
	}
	for _, c := range cases {
		got := run{Status: c.status, Conclusion: c.conclusion}.state(c.isReady)
		if got != c.want {
			t.Errorf("%s/%s ready=%v: got %s, want %s", c.status, c.conclusion, c.isReady, got, c.want)
		}
	}
}

func TestListKeepsOnlySandboxRunsAndFindsReadiness(t *testing.T) {
	github := &fakeGitHub{responses: map[string]string{
		runsPath: `{"workflow_runs":[` + strings.Join([]string{
			runJSON(3, "sbx_ready234", "in_progress", ""),
			runJSON(2, "sbx_start234", "in_progress", ""),
			runJSON(1, "sbx_ended234", "completed", "success"),
			runJSON(9, "Not a sandbox", "in_progress", ""),
		}, ",") + `]}`,
		"GET repos/o/r/actions/runs/3/jobs": `{"jobs":[{"steps":[{"name":"Start SSH and the tunnel","status":"completed"},{"name":"Run until the auto-stop time","status":"in_progress"}]}]}`,
		"GET repos/o/r/actions/runs/2/jobs": `{"jobs":[{"steps":[{"name":"Start SSH and the tunnel","status":"in_progress"}]}]}`,
	}}
	backend := &Backend{Repo: "o/r", Ref: "main", API: github.api}

	machines, err := backend.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	states := map[string]sandbox.State{}
	for _, machine := range machines {
		states[machine.ID] = machine.State
	}
	want := map[string]sandbox.State{
		"sbx_ready234": sandbox.StateReady,
		"sbx_start234": sandbox.StateStarting,
		"sbx_ended234": sandbox.StateStopped,
	}
	if fmt.Sprint(states) != fmt.Sprint(want) {
		t.Errorf("got %v, want %v", states, want)
	}
}

func TestStartRefusesPastTheMacJobLimit(t *testing.T) {
	active := []string{}
	for i := range macJobLimit {
		active = append(active, runJSON(i+1, fmt.Sprintf("sbx_aaaaaaa%d", i+2), "in_progress", ""))
	}
	github := &fakeGitHub{responses: map[string]string{runsPath: `{"workflow_runs":[` + strings.Join(active, ",") + `]}`}}
	backend := &Backend{Repo: "o/r", Ref: "main", API: github.api}

	_, err := backend.Start(context.Background(), sandbox.StartInput{ID: "sbx_new23456", TTL: time.Minute})

	var expected *sandbox.Error
	if !errors.As(err, &expected) || expected.Code != sandbox.CodeLimitReached {
		t.Fatalf("got %v, want %s", err, sandbox.CodeLimitReached)
	}
	if len(github.calls) != 1 {
		t.Errorf("nothing should be dispatched, calls were %v", github.calls)
	}
}

func TestStartDispatchesOnlyPublicValues(t *testing.T) {
	dispatchPath := "POST repos/o/r/actions/workflows/sandbox.yml/dispatches"
	github := &fakeGitHub{responses: map[string]string{
		runsPath:     `{"workflow_runs":[]}`,
		dispatchPath: `{"workflow_run_id":77,"html_url":"https://github.com/o/r/actions/runs/77"}`,
	}}
	backend := &Backend{Repo: "o/r", Ref: "main", API: github.api}

	machine, err := backend.Start(context.Background(), sandbox.StartInput{
		ID:            "sbx_new23456",
		SSHPublicKey:  "ssh-ed25519 AAAA",
		SealPublicKey: "c2VhbA==",
		TTL:           90 * time.Second,
		Transport:     "cloudflare-quick",
	})
	if err != nil {
		t.Fatal(err)
	}
	if machine.Ref != "77" || machine.State != sandbox.StateStarting {
		t.Errorf("got %+v", machine)
	}

	sent, err := json.Marshal(github.bodies[1])
	if err != nil {
		t.Fatal(err)
	}
	want := `{"inputs":{"id":"sbx_new23456","seal_public_key":"c2VhbA==","ssh_public_key":"ssh-ed25519 AAAA","transport":"cloudflare-quick","ttl_seconds":"90"},"ref":"main","return_run_details":true}`
	if string(sent) != want {
		t.Errorf("dispatched %s", sent)
	}
}

func TestAwaitAnnouncementWaitsForARunGitHubDoesNotShowYet(t *testing.T) {
	artifactsPath := "GET repos/o/r/actions/runs/5/artifacts?name=sbx_abcd2345"
	github := &fakeGitHub{
		notFoundAt: map[string]int{artifactsPath: 2},
		responses: map[string]string{
			artifactsPath:                            `{"artifacts":[{"id":44}]}`,
			"GET repos/o/r/actions/artifacts/44/zip": announcementArchive(),
		},
	}
	backend := &Backend{Repo: "o/r", Ref: "main", PollInterval: time.Millisecond, API: github.api}

	announcement, err := backend.AwaitAnnouncement(context.Background(), sandbox.Machine{ID: "sbx_abcd2345", Ref: "5"})
	if err != nil {
		t.Fatal(err)
	}
	if announcement.ID != "sbx_abcd2345" || len(github.calls) != 4 {
		t.Errorf("got %+v after %v", announcement, github.calls)
	}
}

func TestAwaitAnnouncementFailsWhenTheRunEndsFirst(t *testing.T) {
	github := &fakeGitHub{responses: map[string]string{
		"GET repos/o/r/actions/runs/5/artifacts?name=sbx_abcd2345": `{"artifacts":[]}`,
		"GET repos/o/r/actions/runs/5":                             runJSON(5, "sbx_abcd2345", "completed", "failure"),
	}}
	backend := &Backend{Repo: "o/r", Ref: "main", PollInterval: time.Millisecond, API: github.api}

	_, err := backend.AwaitAnnouncement(context.Background(), sandbox.Machine{ID: "sbx_abcd2345", Ref: "5"})

	var expected *sandbox.Error
	if !errors.As(err, &expected) || expected.Code != sandbox.CodeStartFailed {
		t.Fatalf("got %v, want %s", err, sandbox.CodeStartFailed)
	}
}

func TestStopCancelsTheRunAndWaitsForItToEnd(t *testing.T) {
	github := &fakeGitHub{responses: map[string]string{
		"POST repos/o/r/actions/runs/5/cancel": `{}`,
		"GET repos/o/r/actions/runs/5":         runJSON(5, "sbx_ready234", "completed", "cancelled"),
	}}
	backend := &Backend{Repo: "o/r", Ref: "main", PollInterval: time.Millisecond, API: github.api}

	machine, err := backend.Stop(context.Background(), sandbox.Machine{ID: "sbx_ready234", Ref: "5", State: sandbox.StateReady})
	if err != nil {
		t.Fatal(err)
	}
	if machine.State != sandbox.StateStopped || len(github.calls) != 2 {
		t.Errorf("got %+v after %v", machine, github.calls)
	}
}

func TestStopKeepsCancellingARunGitHubDoesNotShowYet(t *testing.T) {
	github := &fakeGitHub{
		notFoundAt: map[string]int{"POST repos/o/r/actions/runs/5/cancel": 2, "GET repos/o/r/actions/runs/5": 2},
		responses: map[string]string{
			"POST repos/o/r/actions/runs/5/cancel": `{}`,
			"GET repos/o/r/actions/runs/5":         runJSON(5, "sbx_ready234", "completed", "cancelled"),
		},
	}
	backend := &Backend{Repo: "o/r", Ref: "main", PollInterval: time.Millisecond, API: github.api}

	machine, err := backend.Stop(context.Background(), sandbox.Machine{ID: "sbx_ready234", Ref: "5", State: sandbox.StateStarting})
	if err != nil {
		t.Fatal(err)
	}
	if machine.State != sandbox.StateStopped || len(github.calls) != 6 {
		t.Errorf("got %+v after %v", machine, github.calls)
	}
}

func TestStopLeavesAStoppedSandboxAlone(t *testing.T) {
	github := &fakeGitHub{}
	backend := &Backend{Repo: "o/r", Ref: "main", API: github.api}

	machine, err := backend.Stop(context.Background(), sandbox.Machine{ID: "sbx_ended234", Ref: "1", State: sandbox.StateStopped})
	if err != nil {
		t.Fatal(err)
	}
	if machine.State != sandbox.StateStopped || len(github.calls) != 0 {
		t.Errorf("got %+v after %v", machine, github.calls)
	}
}

func TestReadAnnouncement(t *testing.T) {
	announcement, err := readAnnouncement([]byte(announcementArchive()))
	if err != nil {
		t.Fatal(err)
	}
	if announcement.ID != "sbx_abcd2345" || announcement.User != "runner" || announcement.SealedEndpoint != "c2VhbGVk" {
		t.Errorf("got %+v", announcement)
	}
	if announcement.HardStopAt.Sub(announcement.AutoStopAt) != 330*time.Minute {
		t.Errorf("stop times are %s and %s", announcement.AutoStopAt, announcement.HardStopAt)
	}

	if _, err := readAnnouncement([]byte("not a zip")); err == nil {
		t.Error("a broken archive was accepted")
	}
}
