package launch

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
)

func WindowsUserName() string {
	if name := os.Getenv("USER"); name != "" {
		return name
	}
	current, err := user.Current()
	if err != nil {
		return "user"
	}
	return current.Username
}

func Places(prefix, gameDir string) map[string]string {
	profile := filepath.Join(prefix, "drive_c", "users", WindowsUserName())
	return map[string]string{
		"{game}":         gameDir,
		"{userprofile}":  profile,
		"{appdata}":      filepath.Join(profile, "AppData", "Roaming"),
		"{localappdata}": filepath.Join(profile, "AppData", "Local"),
		"{documents}":    filepath.Join(profile, "Documents"),
	}
}

func WriteFiles(files []fixes.File, places map[string]string) error {
	for _, file := range files {
		place, rest, _ := strings.Cut(file.Path, "/")
		root, ok := places[place]
		if !ok {
			return failure.Errorf(failure.CodeBadProfile, "file %q is in no known place", file.Path)
		}
		path := filepath.Join(root, filepath.FromSlash(rest))
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
