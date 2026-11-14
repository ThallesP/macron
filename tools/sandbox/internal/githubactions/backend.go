package githubactions

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

const (
	workflowFile     = "sandbox.yml"
	announcementFile = "sandbox.json"
	readyStepName    = "Run until the auto-stop time"
	macJobLimit      = 5
)

type Backend struct {
	Repo         string
	Ref          string
	PollInterval time.Duration
	API          func(ctx context.Context, method, path string, body any) ([]byte, error)
}

type run struct {
	ID           int64     `json:"id"`
	DisplayTitle string    `json:"display_title"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	CreatedAt    time.Time `json:"created_at"`
	HTMLURL      string    `json:"html_url"`
}

func (r run) isCompleted() bool {
	return r.Status == "completed"
}

func (r run) machine(isReady bool) sandbox.Machine {
	return sandbox.Machine{
		ID:        r.DisplayTitle,
		Ref:       strconv.FormatInt(r.ID, 10),
		State:     r.state(isReady),
		CreatedAt: r.CreatedAt,
		URL:       r.HTMLURL,
	}
}

func (r run) state(isReady bool) sandbox.State {
	if !r.isCompleted() && isReady {
		return sandbox.StateReady
	}
	if !r.isCompleted() {
		return sandbox.StateStarting
	}

	switch r.Conclusion {
	case "success", "cancelled", "timed_out":
		return sandbox.StateStopped
	default:
		return sandbox.StateFailed
	}
}

func (*Backend) Name() string {
	return "github-actions"
}

func (b *Backend) Start(ctx context.Context, in sandbox.StartInput) (sandbox.Machine, error) {
	runs, err := b.runs(ctx)
	if err != nil {
		return sandbox.Machine{}, err
	}
	active := 0
	for _, r := range runs {
		if !r.isCompleted() {
			active++
		}
	}
	if active >= macJobLimit {
		return sandbox.Machine{}, sandbox.Errorf(sandbox.CodeLimitReached, "%d sandboxes are already running, which is GitHub's limit for Mac jobs; stop one first", active)
	}

	var dispatched struct {
		RunID   int64  `json:"workflow_run_id"`
		HTMLURL string `json:"html_url"`
	}
	err = b.call(ctx, "POST", fmt.Sprintf("repos/%s/actions/workflows/%s/dispatches", b.Repo, workflowFile), map[string]any{
		"ref":                b.Ref,
		"return_run_details": true,
		"inputs": map[string]string{
			"id":              in.ID,
			"ssh_public_key":  in.SSHPublicKey,
			"seal_public_key": in.SealPublicKey,
			"ttl_seconds":     strconv.Itoa(int(in.TTL.Seconds())),
			"transport":       in.Transport,
		},
	}, &dispatched)
	if err != nil {
		return sandbox.Machine{}, err
	}
	if dispatched.RunID == 0 {
		return sandbox.Machine{}, fmt.Errorf("GitHub started the sandbox workflow but did not say which run it is")
	}

	return sandbox.Machine{
		ID:        in.ID,
		Ref:       strconv.FormatInt(dispatched.RunID, 10),
		State:     sandbox.StateStarting,
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		URL:       dispatched.HTMLURL,
	}, nil
}

func (b *Backend) AwaitAnnouncement(ctx context.Context, machine sandbox.Machine) (sandbox.Announcement, error) {
	for {
		announcement, isAnnounced, err := b.pollAnnouncement(ctx, machine)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return sandbox.Announcement{}, err
		}
		if isAnnounced {
			return announcement, nil
		}

		select {
		case <-ctx.Done():
			return sandbox.Announcement{}, ctx.Err()
		case <-time.After(b.PollInterval):
		}
	}
}

func (b *Backend) pollAnnouncement(ctx context.Context, machine sandbox.Machine) (sandbox.Announcement, bool, error) {
	var listed struct {
		Artifacts []struct {
			ID int64 `json:"id"`
		} `json:"artifacts"`
	}
	err := b.call(ctx, "GET", fmt.Sprintf("repos/%s/actions/runs/%s/artifacts?name=%s", b.Repo, machine.Ref, machine.ID), nil, &listed)
	if err != nil {
		return sandbox.Announcement{}, false, err
	}
	if len(listed.Artifacts) > 0 {
		announcement, err := b.announcement(ctx, listed.Artifacts[0].ID)
		return announcement, err == nil, err
	}

	var current run
	if err := b.call(ctx, "GET", fmt.Sprintf("repos/%s/actions/runs/%s", b.Repo, machine.Ref), nil, &current); err != nil {
		return sandbox.Announcement{}, false, err
	}
	if current.isCompleted() {
		return sandbox.Announcement{}, false, sandbox.Errorf(sandbox.CodeStartFailed, "%s ended before it was ready (%s); see %s", machine.ID, current.Conclusion, current.HTMLURL)
	}
	return sandbox.Announcement{}, false, nil
}

func (b *Backend) announcement(ctx context.Context, artifactID int64) (sandbox.Announcement, error) {
	archive, err := b.API(ctx, "GET", fmt.Sprintf("repos/%s/actions/artifacts/%d/zip", b.Repo, artifactID), nil)
	if err != nil {
		return sandbox.Announcement{}, err
	}
	return readAnnouncement(archive)
}

func readAnnouncement(archive []byte) (sandbox.Announcement, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return sandbox.Announcement{}, fmt.Errorf("read the sandbox announcement: %w", err)
	}
	file, err := reader.Open(announcementFile)
	if err != nil {
		return sandbox.Announcement{}, fmt.Errorf("read the sandbox announcement: %w", err)
	}
	defer file.Close()

	var announced struct {
		ID             string `json:"id"`
		Transport      string `json:"transport"`
		User           string `json:"user"`
		HostKey        string `json:"hostKey"`
		SealedEndpoint string `json:"sealedEndpoint"`
		AutoStopAtUnix int64  `json:"autoStopAt"`
		HardStopAtUnix int64  `json:"hardStopAt"`
	}
	if err := json.NewDecoder(io.LimitReader(file, 64*1024)).Decode(&announced); err != nil {
		return sandbox.Announcement{}, fmt.Errorf("read the sandbox announcement: %w", err)
	}

	return sandbox.Announcement{
		ID:             announced.ID,
		Transport:      announced.Transport,
		User:           announced.User,
		HostKey:        announced.HostKey,
		SealedEndpoint: announced.SealedEndpoint,
		AutoStopAt:     time.Unix(announced.AutoStopAtUnix, 0).UTC(),
		HardStopAt:     time.Unix(announced.HardStopAtUnix, 0).UTC(),
	}, nil
}

func (b *Backend) Find(ctx context.Context, id string) (sandbox.Machine, bool, error) {
	runs, err := b.runs(ctx)
	if err != nil {
		return sandbox.Machine{}, false, err
	}

	for _, r := range runs {
		if r.DisplayTitle != id {
			continue
		}
		machine, err := b.machine(ctx, r)
		return machine, err == nil, err
	}
	return sandbox.Machine{}, false, nil
}

func (b *Backend) List(ctx context.Context) ([]sandbox.Machine, error) {
	runs, err := b.runs(ctx)
	if err != nil {
		return nil, err
	}

	machines := make([]sandbox.Machine, 0, len(runs))
	for _, r := range runs {
		machine, err := b.machine(ctx, r)
		if err != nil {
			return nil, err
		}
		machines = append(machines, machine)
	}
	return machines, nil
}

func (b *Backend) Stop(ctx context.Context, machine sandbox.Machine) (sandbox.Machine, error) {
	if !machine.State.IsActive() {
		return machine, nil
	}

	runPath := fmt.Sprintf("repos/%s/actions/runs/%s", b.Repo, machine.Ref)
	isCancelRequested := false
	for {
		var cancelErr error
		if !isCancelRequested {
			cancelErr = b.call(ctx, "POST", runPath+"/cancel", nil, nil)
			isCancelRequested = cancelErr == nil
		}

		var current run
		err := b.call(ctx, "GET", runPath, nil, &current)
		if err == nil && current.isCompleted() {
			return current.machine(false), nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return sandbox.Machine{}, err
		}
		if cancelErr != nil && !errors.Is(cancelErr, ErrNotFound) {
			return sandbox.Machine{}, cancelErr
		}

		select {
		case <-ctx.Done():
			return sandbox.Machine{}, fmt.Errorf("%s was told to stop but has not ended; see %s", machine.ID, machine.URL)
		case <-time.After(b.PollInterval):
		}
	}
}

func (b *Backend) runs(ctx context.Context) ([]run, error) {
	var listed struct {
		Runs []run `json:"workflow_runs"`
	}
	path := fmt.Sprintf("repos/%s/actions/workflows/%s/runs?event=workflow_dispatch&per_page=100", b.Repo, workflowFile)
	if err := b.call(ctx, "GET", path, nil, &listed); err != nil {
		return nil, err
	}

	sandboxes := make([]run, 0, len(listed.Runs))
	for _, r := range listed.Runs {
		if sandbox.IsID(r.DisplayTitle) {
			sandboxes = append(sandboxes, r)
		}
	}
	return sandboxes, nil
}

func (b *Backend) machine(ctx context.Context, r run) (sandbox.Machine, error) {
	if r.isCompleted() {
		return r.machine(false), nil
	}

	var listed struct {
		Jobs []struct {
			Steps []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"steps"`
		} `json:"jobs"`
	}
	err := b.call(ctx, "GET", fmt.Sprintf("repos/%s/actions/runs/%d/jobs", b.Repo, r.ID), nil, &listed)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return sandbox.Machine{}, err
	}

	for _, job := range listed.Jobs {
		for _, step := range job.Steps {
			if step.Name == readyStepName && step.Status == "in_progress" {
				return r.machine(true), nil
			}
		}
	}
	return r.machine(false), nil
}

func (b *Backend) call(ctx context.Context, method, path string, body, into any) error {
	response, err := b.API(ctx, method, path, body)
	if err != nil {
		return err
	}
	if into == nil {
		return nil
	}
	if err := json.Unmarshal(response, into); err != nil {
		return fmt.Errorf("GitHub %s %s answered with unreadable JSON: %w", method, path, err)
	}
	return nil
}
