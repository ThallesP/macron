package fixes

import (
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/ThallesP/macron/internal/failure"
)

type Backend string

const (
	BackendDXMT    Backend = "dxmt"
	BackendWineD3D Backend = "wined3d"
)

func (b Backend) IsKnown() bool {
	return b == BackendDXMT || b == BackendWineD3D
}

type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type Profile struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	SteamAppIDs  []int             `json:"steamAppIds"`
	Backend      Backend           `json:"backend"`
	Args         []string          `json:"args"`
	Env          map[string]string `json:"env"`
	DLLOverrides map[string]string `json:"dllOverrides"`
	Files        []File            `json:"files"`
	Installs     []string          `json:"installs"`
	Notes        string            `json:"notes"`
}

type Database struct {
	profiles []Profile
}

//go:embed games/*.json
var games embed.FS

var (
	validID          = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	validDLLOverride = regexp.MustCompile(`^(native|builtin|n|b|n,b|b,n|disabled|)$`)
	FilePlaces       = []string{"{game}", "{appdata}", "{localappdata}", "{documents}", "{userprofile}"}
)

func Load() (Database, error) {
	entries, err := games.ReadDir("games")
	if err != nil {
		return Database{}, err
	}
	var db Database
	for _, entry := range entries {
		content, err := games.ReadFile(path.Join("games", entry.Name()))
		if err != nil {
			return Database{}, err
		}
		profile, err := Parse(content)
		if err != nil {
			return Database{}, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if strings.TrimSuffix(entry.Name(), ".json") != profile.ID {
			return Database{}, failure.Errorf(failure.CodeBadProfile, "%s holds profile %q; the file name must match the id", entry.Name(), profile.ID)
		}
		db.profiles = append(db.profiles, profile)
	}
	return db, nil
}

func Parse(content []byte) (Profile, error) {
	var profile Profile
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return Profile{}, failure.Errorf(failure.CodeBadProfile, "bad profile: %s", err)
	}
	if err := profile.validate(); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (p Profile) validate() error {
	if !validID.MatchString(p.ID) {
		return failure.Errorf(failure.CodeBadProfile, "profile id %q must be lowercase letters, digits and dashes", p.ID)
	}
	if !p.Backend.IsKnown() {
		return failure.Errorf(failure.CodeBadProfile, "profile %s: unknown backend %q", p.ID, p.Backend)
	}
	for dll, mode := range p.DLLOverrides {
		if !validDLLOverride.MatchString(mode) {
			return failure.Errorf(failure.CodeBadProfile, "profile %s: DLL override %s=%q is not a Wine load order", p.ID, dll, mode)
		}
	}
	for _, file := range p.Files {
		if !slices.ContainsFunc(FilePlaces, func(place string) bool { return strings.HasPrefix(file.Path, place+"/") }) {
			return failure.Errorf(failure.CodeBadProfile, "profile %s: file %q must start with one of %s", p.ID, file.Path, strings.Join(FilePlaces, ", "))
		}
		if slices.Contains(strings.Split(file.Path, "/"), "..") {
			return failure.Errorf(failure.CodeBadProfile, "profile %s: file %q leaves its folder", p.ID, file.Path)
		}
	}
	if len(p.Installs) > 0 {
		return failure.Errorf(failure.CodeBadProfile, "profile %s: extra installs are not supported yet (%s)", p.ID, strings.Join(p.Installs, ", "))
	}
	return nil
}

func (db Database) Get(id string) (Profile, error) {
	index := slices.IndexFunc(db.profiles, func(p Profile) bool { return p.ID == id })
	if index < 0 {
		return Profile{}, failure.Errorf(failure.CodeUnknownGame, "no profile %q in the fixes database", id)
	}
	return db.profiles[index], nil
}

func (db Database) ForSteamApp(appID int) Profile {
	index := slices.IndexFunc(db.profiles, func(p Profile) bool { return slices.Contains(p.SteamAppIDs, appID) })
	if index < 0 {
		return Default(fmt.Sprintf("steam-%d", appID))
	}
	return db.profiles[index]
}

func Default(id string) Profile {
	return Profile{ID: id, Name: id, Backend: BackendDXMT}
}
