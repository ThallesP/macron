package sandbox

import (
	"context"
	"os/exec"
	"time"
)

const (
	DefaultTTL   = 30 * time.Minute
	MinTTL       = 30 * time.Second
	MaxTTL       = 330 * time.Minute
	StartTimeout = 10 * time.Minute
	StopTimeout  = 3 * time.Minute
)

type App struct {
	Backend   Backend
	Transport Transport
	Store     Store
}

func (a *App) checkTools() error {
	for _, tool := range []string{"ssh", "scp", "ssh-keygen"} {
		if _, err := exec.LookPath(tool); err != nil {
			return Errorf(CodeMissingTool, "%s is not installed; sbx needs OpenSSH", tool)
		}
	}
	return nil
}

func checkTTL(ttl time.Duration) error {
	if ttl < MinTTL || ttl > MaxTTL {
		return Errorf(CodeInvalidInput, "--ttl must be between %s and %s", MinTTL, MaxTTL)
	}
	return nil
}

func (a *App) describe(machine Machine) (Sandbox, error) {
	record, found, err := a.Store.Load(machine.ID)
	if err != nil {
		return Sandbox{}, err
	}
	return describe(machine, a.Backend.Name(), record, found), nil
}

func (a *App) connectable(ctx context.Context, id string) (Record, error) {
	if !IsID(id) {
		return Record{}, Errorf(CodeInvalidInput, "%q is not a sandbox ID (expected sbx_ and 8 characters)", id)
	}

	record, found, err := a.Store.Load(id)
	if err != nil {
		return Record{}, err
	}
	if found && record.Ready {
		return record, nil
	}
	if found {
		return Record{}, Errorf(CodeNotReady, "%s is still starting", id)
	}

	machine, exists, err := a.Backend.Find(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if !exists {
		return Record{}, Errorf(CodeNotFound, "no sandbox %s", id)
	}
	if !machine.State.IsActive() {
		return Record{}, Errorf(CodeStopped, "%s has stopped", id)
	}
	return Record{}, Errorf(CodeKeyNotHere, "%s was created on another machine; only that machine holds its SSH key", id)
}

func (a *App) stoppedError(ctx context.Context, id string) error {
	machine, exists, err := a.Backend.Find(ctx, id)
	if err != nil || !exists || machine.State.IsActive() {
		return nil
	}
	if err := a.Store.Remove(id); err != nil {
		return err
	}
	return Errorf(CodeStopped, "%s has stopped", id)
}
