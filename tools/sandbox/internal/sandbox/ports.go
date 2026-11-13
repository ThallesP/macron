package sandbox

import (
	"context"
	"fmt"
	"regexp"
	"time"
)

type StartInput struct {
	ID            string
	SSHPublicKey  string
	SealPublicKey string
	TTL           time.Duration
	Transport     string
}

type Announcement struct {
	ID             string
	Transport      string
	User           string
	HostKey        string
	SealedEndpoint string
	AutoStopAt     time.Time
	HardStopAt     time.Time
}

var (
	userPattern    = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	hostKeyPattern = regexp.MustCompile(`^ssh-ed25519 [A-Za-z0-9+/]{68}$`)
)

func (a Announcement) Validate(id, transport string) error {
	if a.ID != id {
		return fmt.Errorf("the announcement is for %q, not %s", a.ID, id)
	}
	if a.Transport != transport {
		return fmt.Errorf("the sandbox uses transport %q, this CLI asked for %s", a.Transport, transport)
	}
	if !userPattern.MatchString(a.User) {
		return fmt.Errorf("the sandbox announced an invalid user name %q", a.User)
	}
	if !hostKeyPattern.MatchString(a.HostKey) {
		return fmt.Errorf("the sandbox announced an invalid SSH host key")
	}
	return nil
}

type Backend interface {
	Name() string
	Start(ctx context.Context, in StartInput) (Machine, error)
	AwaitAnnouncement(ctx context.Context, machine Machine) (Announcement, error)
	Find(ctx context.Context, id string) (Machine, bool, error)
	List(ctx context.Context) ([]Machine, error)
	Stop(ctx context.Context, id string) (Machine, error)
}

type Route struct {
	HostName     string
	ProxyCommand string
}

type Transport interface {
	Name() string
	Route(endpoint string) (Route, error)
}
