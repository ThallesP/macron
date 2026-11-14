package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type Record struct {
	ID         string    `json:"id"`
	Transport  string    `json:"transport"`
	Ready      bool      `json:"ready"`
	AutoStopAt time.Time `json:"autoStopAt"`
	HardStopAt time.Time `json:"hardStopAt"`
}

type Store struct {
	Dir string
}

func (s Store) KeyPath(id string) string {
	return filepath.Join(s.Dir, id, "id_ed25519")
}

func (s Store) KnownHostsPath(id string) string {
	return filepath.Join(s.Dir, id, "known_hosts")
}

func (s Store) SSHConfigPath(id string) string {
	return filepath.Join(s.Dir, id, "ssh_config")
}

func (s Store) recordPath(id string) string {
	return filepath.Join(s.Dir, id, "sandbox.json")
}

func (s Store) Create(id string) error {
	return os.MkdirAll(filepath.Join(s.Dir, id), 0o700)
}

func (s Store) Save(record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.recordPath(record.ID), data, 0o600)
}

func (s Store) Load(id string) (Record, bool, error) {
	data, err := os.ReadFile(s.recordPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return Record{}, false, nil
	}
	if err != nil {
		return Record{}, false, err
	}

	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, false, fmt.Errorf("read %s: %w", s.recordPath(id), err)
	}
	return record, true, nil
}

func (s Store) Remove(id string) error {
	return os.RemoveAll(filepath.Join(s.Dir, id))
}
