package steamhook

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ThallesP/macron/internal/failure"
)

const ToolName = "macron"

var identityFiles = []string{"steamclient.dylib", "steam_osx"}

var mappingPath = []string{"InstallConfigStore", "Software", "Valve", "Steam", "CompatToolMapping"}

type Hook struct {
	SteamDir       string
	MacronPath     string
	IsSteamRunning func() bool
}

type Build struct {
	ID      string            `json:"id"`
	IsKnown bool              `json:"isKnown"`
	SHA256  map[string]string `json:"sha256"`
}

type Status struct {
	SteamDir         string `json:"steamDir"`
	Build            Build  `json:"build"`
	IsToolRegistered bool   `json:"isToolRegistered"`
	IsDefaultTool    bool   `json:"isDefaultTool"`
	HasBackup        bool   `json:"hasBackup"`
}

func (s Status) IsEnabled() bool {
	return s.IsToolRegistered && s.IsDefaultTool
}

func (h Hook) clientDir() string {
	return filepath.Join(h.SteamDir, "Steam.AppBundle", "Steam", "Contents", "MacOS")
}

func (h Hook) toolDir() string {
	return filepath.Join(h.SteamDir, "compatibilitytools.d", ToolName)
}

func (h Hook) configPath() string {
	return filepath.Join(h.SteamDir, "config", "config.vdf")
}

func (h Hook) backupPath() string {
	return h.configPath() + ".before-macron"
}

func (h Hook) Status() (Status, error) {
	build, err := h.Identify()
	if err != nil {
		return Status{}, err
	}

	config, err := h.readConfig(h.configPath())
	if err != nil {
		return Status{}, err
	}

	return Status{
		SteamDir:         h.SteamDir,
		Build:            build,
		IsToolRegistered: fileExists(filepath.Join(h.toolDir(), "compatibilitytool.vdf")),
		IsDefaultTool:    isMacronMapping(config),
		HasBackup:        fileExists(h.backupPath()),
	}, nil
}

func (h Hook) Identify() (Build, error) {
	if !fileExists(filepath.Join(h.clientDir(), "steamclient.dylib")) {
		return Build{}, failure.Errorf(failure.CodeSteamNotFound, "no Steam client in %s; start Steam once so it downloads the client", h.SteamDir)
	}

	hashes := map[string]string{}
	for _, name := range identityFiles {
		sum, err := sha256File(filepath.Join(h.clientDir(), name))
		if err != nil {
			return Build{}, err
		}
		hashes[name] = sum
	}

	build := Build{ID: h.manifestVersion(), SHA256: hashes}
	build.IsKnown = slices.ContainsFunc(KnownBuilds, func(known KnownBuild) bool {
		return known.matches(hashes)
	})
	return build, nil
}

func (k KnownBuild) matches(hashes map[string]string) bool {
	for _, file := range k.Files {
		if hashes[file.Path] != file.SHA256 {
			return false
		}
	}
	return len(k.Files) > 0
}

func (h Hook) manifestVersion() string {
	data, err := os.ReadFile(filepath.Join(h.clientDir(), "package", "steam_client_osx.manifest"))
	if err != nil {
		return "unknown"
	}

	manifest, err := ParseVDF(string(data))
	if err != nil {
		return "unknown"
	}

	version := manifest.Path("osx", "version")
	if version == nil {
		return "unknown"
	}
	return version.Value
}

func (h Hook) Enable() (Status, error) {
	if !filepath.IsAbs(h.MacronPath) {
		return Status{}, failure.Errorf(failure.CodeInvalidInput, "the macron path %q is not absolute", h.MacronPath)
	}

	status, err := h.Status()
	if err != nil {
		return Status{}, err
	}
	if !status.Build.IsKnown {
		return Status{}, unknownBuildError(status.Build)
	}
	if h.IsSteamRunning() {
		return Status{}, failure.Errorf(failure.CodeSteamRunning, "Steam is running; quit it first, it rewrites config.vdf when it exits")
	}

	config, err := h.readConfig(h.configPath())
	if err != nil {
		return Status{}, err
	}
	if err := os.MkdirAll(filepath.Dir(h.configPath()), 0o755); err != nil {
		return Status{}, failure.Errorf(failure.CodeFileFailed, "could not create %s: %v", filepath.Dir(h.configPath()), err)
	}
	if !status.IsDefaultTool {
		if err := h.writeBackup(); err != nil {
			return Status{}, err
		}
	}
	if err := h.writeTool(); err != nil {
		return Status{}, err
	}

	config.EnsureObject(mappingPath...).Set(&Node{Key: "0", IsObject: true, Children: []*Node{
		{Key: "name", Value: ToolName},
		{Key: "config", Value: ""},
		{Key: "priority", Value: "75"},
	}})
	if err := writeFileAtomic(h.configPath(), config.String()); err != nil {
		return Status{}, err
	}
	return h.Status()
}

func (h Hook) Disable() (Status, error) {
	if h.IsSteamRunning() {
		return Status{}, failure.Errorf(failure.CodeSteamRunning, "Steam is running; quit it first, it rewrites config.vdf when it exits")
	}

	config, err := h.readConfig(h.configPath())
	if err != nil {
		return Status{}, err
	}
	isRegistered := fileExists(h.toolDir())
	if !isRegistered && !isMacronMapping(config) {
		return Status{}, failure.Errorf(failure.CodeHookNotEnabled, "the Steam hook is not enabled in %s", h.SteamDir)
	}

	if isMacronMapping(config) {
		if err := h.restoreMapping(config); err != nil {
			return Status{}, err
		}
	}
	if err := os.RemoveAll(h.toolDir()); err != nil {
		return Status{}, failure.Errorf(failure.CodeFileFailed, "could not remove %s: %v", h.toolDir(), err)
	}
	toolsDir := filepath.Dir(h.toolDir())
	if entries, err := os.ReadDir(toolsDir); err == nil && len(entries) == 0 {
		os.Remove(toolsDir)
	}
	if err := os.Remove(h.backupPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Status{}, failure.Errorf(failure.CodeFileFailed, "could not remove %s: %v", h.backupPath(), err)
	}
	return h.Status()
}

func (h Hook) restoreMapping(config *Node) error {
	backup, err := h.readConfig(h.backupPath())
	if err != nil {
		return err
	}

	mappings := config.EnsureObject(mappingPath...)
	original := backup.Path(append(mappingPath, "0")...)
	if original != nil {
		mappings.Set(original)
	}
	if original == nil {
		mappings.Remove("0")
	}
	if len(mappings.Children) == 0 {
		config.Path(mappingPath[:len(mappingPath)-1]...).Remove(mappingPath[len(mappingPath)-1])
	}
	return writeFileAtomic(h.configPath(), config.String())
}

func (h Hook) writeBackup() error {
	data, err := os.ReadFile(h.configPath())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failure.Errorf(failure.CodeFileFailed, "could not read %s: %v", h.configPath(), err)
	}
	return writeFileAtomic(h.backupPath(), string(data))
}

func (h Hook) writeTool() error {
	if err := os.MkdirAll(h.toolDir(), 0o755); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not create %s: %v", h.toolDir(), err)
	}
	if err := writeFileAtomic(filepath.Join(h.toolDir(), "compatibilitytool.vdf"), compatibilityToolVDF); err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(h.toolDir(), "toolmanifest.vdf"), toolManifestVDF); err != nil {
		return err
	}

	launcher := filepath.Join(h.toolDir(), ToolName)
	if err := os.Remove(launcher); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failure.Errorf(failure.CodeFileFailed, "could not replace %s: %v", launcher, err)
	}
	if err := os.Symlink(h.MacronPath, launcher); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not link %s to %s: %v", launcher, h.MacronPath, err)
	}
	return nil
}

func (h Hook) readConfig(path string) (*Node, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Node{IsObject: true}, nil
	}
	if err != nil {
		return nil, failure.Errorf(failure.CodeFileFailed, "could not read %s: %v", path, err)
	}
	return ParseVDF(string(data))
}

func isMacronMapping(config *Node) bool {
	name := config.Path(append(mappingPath, "0", "name")...)
	return name != nil && name.Value == ToolName
}

func unknownBuildError(build Build) *failure.Error {
	var hashes []string
	for _, name := range identityFiles {
		hashes = append(hashes, name+" sha256 "+build.SHA256[name])
	}
	return failure.Errorf(failure.CodeUnknownBuild,
		"Steam build %s is not in macron's table of known builds (%s); nothing was changed. README.md \"Steam hook\" says how to add a build",
		build.ID, strings.Join(hashes, ", "))
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", failure.Errorf(failure.CodeSteamNotFound, "could not open %s: %v", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", failure.Errorf(failure.CodeFileFailed, "could not read %s: %v", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func writeFileAtomic(path string, content string) error {
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not write %s: %v", path, err)
	}
	defer os.Remove(temp.Name())

	if _, err := temp.WriteString(content); err != nil {
		temp.Close()
		return failure.Errorf(failure.CodeFileFailed, "could not write %s: %v", path, err)
	}
	if err := temp.Close(); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not write %s: %v", path, err)
	}
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not write %s: %v", path, err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return failure.Errorf(failure.CodeFileFailed, "could not write %s: %v", path, err)
	}
	return nil
}
