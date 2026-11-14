package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (a *App) Info(ctx context.Context, id string) (Sandbox, error) {
	machine, err := a.find(ctx, id)
	if err != nil {
		return Sandbox{}, err
	}
	if err := a.forgetStopped(machine); err != nil {
		return Sandbox{}, err
	}
	return a.describe(machine)
}

type ListInput struct {
	IncludeStopped bool
}

func (a *App) List(ctx context.Context, in ListInput) ([]Sandbox, error) {
	machines, err := a.Backend.List(ctx)
	if err != nil {
		return nil, err
	}

	sandboxes := []Sandbox{}
	for _, machine := range machines {
		if err := a.forgetStopped(machine); err != nil {
			return nil, err
		}
		if !machine.State.IsActive() && !in.IncludeStopped {
			continue
		}
		sandbox, err := a.describe(machine)
		if err != nil {
			return nil, err
		}
		sandboxes = append(sandboxes, sandbox)
	}
	return sandboxes, nil
}

func (a *App) Stop(ctx context.Context, id string) (Sandbox, error) {
	machine, err := a.find(ctx, id)
	if err != nil {
		return Sandbox{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, StopTimeout)
	defer cancel()

	stopped, err := a.Backend.Stop(ctx, machine)
	if err != nil {
		return Sandbox{}, err
	}
	if err := a.Store.Remove(id); err != nil {
		return Sandbox{}, err
	}
	return a.describe(stopped)
}

type ExtendInput struct {
	ID  string
	TTL time.Duration
}

func (a *App) Extend(ctx context.Context, in ExtendInput) (Sandbox, error) {
	if err := checkTTL(in.TTL); err != nil {
		return Sandbox{}, err
	}
	record, err := a.connectable(ctx, in.ID)
	if err != nil {
		return Sandbox{}, err
	}

	remote := fmt.Sprintf(
		`stop_at=$(( $(date +%%s) + %d )) && echo "$stop_at" > ~/.sbx/auto-stop-at.new && mv ~/.sbx/auto-stop-at.new ~/.sbx/auto-stop-at && echo "$stop_at"`,
		int(in.TTL.Seconds()),
	)
	var output bytes.Buffer
	exitCode, err := a.run(ctx, remoteCommand{ID: in.ID, Line: remote, Stdout: &output})
	if err != nil {
		return Sandbox{}, err
	}
	if exitCode != 0 {
		return Sandbox{}, Errorf(CodeExtendFailed, "%s could not move its auto-stop time", in.ID)
	}
	stopAtUnix, err := strconv.ParseInt(strings.TrimSpace(output.String()), 10, 64)
	if err != nil {
		return Sandbox{}, Errorf(CodeExtendFailed, "%s answered with an unreadable auto-stop time", in.ID)
	}

	record.AutoStopAt = time.Unix(stopAtUnix, 0)
	if record.AutoStopAt.After(record.HardStopAt) {
		record.AutoStopAt = record.HardStopAt
	}
	if err := a.Store.Save(record); err != nil {
		return Sandbox{}, err
	}
	return a.Info(ctx, in.ID)
}

func (a *App) find(ctx context.Context, id string) (Machine, error) {
	if !IsID(id) {
		return Machine{}, Errorf(CodeInvalidInput, "%q is not a sandbox ID (expected sbx_ and 8 characters)", id)
	}

	machine, exists, err := a.Backend.Find(ctx, id)
	if err != nil {
		return Machine{}, err
	}
	if !exists {
		return Machine{}, Errorf(CodeNotFound, "no sandbox %s", id)
	}
	return machine, nil
}

func (a *App) forgetStopped(machine Machine) error {
	if machine.State.IsActive() {
		return nil
	}
	return a.Store.Remove(machine.ID)
}
