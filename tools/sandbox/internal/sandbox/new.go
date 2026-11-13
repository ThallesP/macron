package sandbox

import (
	"context"
	"errors"
	"time"
)

type NewInput struct {
	TTL       time.Duration
	OnStarted func(Sandbox)
}

func (a *App) New(ctx context.Context, in NewInput) (Sandbox, error) {
	if err := checkTTL(in.TTL); err != nil {
		return Sandbox{}, err
	}
	if err := a.checkTools(); err != nil {
		return Sandbox{}, err
	}

	id := newID()
	if err := a.Store.Create(id); err != nil {
		return Sandbox{}, err
	}

	sandbox, err := a.start(ctx, id, in)
	if err != nil {
		a.abandon(id)
		return Sandbox{}, err
	}
	return sandbox, nil
}

func (a *App) start(ctx context.Context, id string, in NewInput) (Sandbox, error) {
	sshPublicKey, err := a.generateKey(ctx, id)
	if err != nil {
		return Sandbox{}, err
	}
	seal, err := newSealKey()
	if err != nil {
		return Sandbox{}, err
	}
	sealPublicKey, err := seal.Public()
	if err != nil {
		return Sandbox{}, err
	}

	machine, err := a.Backend.Start(ctx, StartInput{
		ID:            id,
		SSHPublicKey:  sshPublicKey,
		SealPublicKey: sealPublicKey,
		TTL:           in.TTL,
		Transport:     a.Transport.Name(),
	})
	if err != nil {
		return Sandbox{}, err
	}

	record := Record{ID: id, Transport: a.Transport.Name()}
	if err := a.Store.Save(record); err != nil {
		return Sandbox{}, err
	}
	in.OnStarted(describe(machine, a.Backend.Name(), record, true))

	ctx, cancel := context.WithTimeout(ctx, StartTimeout)
	defer cancel()

	announcement, err := a.Backend.AwaitAnnouncement(ctx, machine)
	if err != nil {
		return Sandbox{}, startError(id, err)
	}
	if err := announcement.Validate(id, a.Transport.Name()); err != nil {
		return Sandbox{}, Errorf(CodeStartFailed, "%s: %s", id, err)
	}

	endpoint, err := seal.Open(announcement.SealedEndpoint)
	if err != nil {
		return Sandbox{}, Errorf(CodeStartFailed, "%s: %s", id, err)
	}
	route, err := a.Transport.Route(endpoint)
	if err != nil {
		return Sandbox{}, Errorf(CodeStartFailed, "%s: %s", id, err)
	}
	if err := a.writeSSHFiles(id, announcement, route); err != nil {
		return Sandbox{}, err
	}
	if err := a.awaitSSH(ctx, id); err != nil {
		return Sandbox{}, startError(id, err)
	}

	record.Ready = true
	record.AutoStopAt = announcement.AutoStopAt
	record.HardStopAt = announcement.HardStopAt
	if err := a.Store.Save(record); err != nil {
		return Sandbox{}, err
	}

	machine.State = StateReady
	return describe(machine, a.Backend.Name(), record, true), nil
}

func startError(id string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return Errorf(CodeStartFailed, "%s was not ready after %s; it has been stopped", id, StartTimeout)
	}
	if errors.Is(err, context.Canceled) {
		return Errorf(CodeStartFailed, "interrupted before %s was ready; it has been stopped", id)
	}
	return err
}

func (a *App) abandon(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), StopTimeout)
	defer cancel()

	a.Backend.Stop(ctx, id)
	a.Store.Remove(id)
}
