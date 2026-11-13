package sandbox

import (
	"crypto/rand"
	"regexp"
	"strings"
	"time"
)

type State string

const (
	StateStarting State = "starting"
	StateReady    State = "ready"
	StateStopped  State = "stopped"
	StateFailed   State = "failed"
)

func (s State) IsActive() bool {
	return s == StateStarting || s == StateReady
}

type Machine struct {
	ID        string
	Ref       string
	State     State
	CreatedAt time.Time
	URL       string
}

type Sandbox struct {
	ID         string     `json:"id"`
	State      State      `json:"state"`
	Backend    string     `json:"backend"`
	Transport  string     `json:"transport,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
	AutoStopAt *time.Time `json:"autoStopAt,omitempty"`
	HardStopAt *time.Time `json:"hardStopAt,omitempty"`
	URL        string     `json:"url"`
	HasKey     bool       `json:"hasKey"`
}

var idPattern = regexp.MustCompile(`^sbx_[a-z0-9]{8}$`)

func IsID(id string) bool {
	return idPattern.MatchString(id)
}

func newID() string {
	return "sbx_" + strings.ToLower(rand.Text())[:8]
}

func describe(machine Machine, backend string, record Record, hasKey bool) Sandbox {
	sandbox := Sandbox{
		ID:        machine.ID,
		State:     machine.State,
		Backend:   backend,
		CreatedAt: machine.CreatedAt,
		URL:       machine.URL,
		HasKey:    hasKey,
	}
	if !hasKey || !record.Ready || !machine.State.IsActive() {
		return sandbox
	}

	sandbox.Transport = record.Transport
	sandbox.AutoStopAt = &record.AutoStopAt
	sandbox.HardStopAt = &record.HardStopAt
	return sandbox
}
