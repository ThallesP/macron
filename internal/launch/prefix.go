package launch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

type prefixRecord struct {
	Runtime string        `json:"runtime"`
	Backend fixes.Backend `json:"backend"`
}

type PrefixInput struct {
	Runtime  wineruntime.Installed
	Backend  fixes.Backend
	Prefix   string
	Env      []string
	Output   io.Writer
	Progress func(string)
}

const prefixMarker = ".macron-prefix.json"

func EnsurePrefix(ctx context.Context, in PrefixInput) error {
	want := prefixRecord{Runtime: in.Runtime.Version, Backend: in.Backend}
	var have prefixRecord
	if recorded, err := os.ReadFile(filepath.Join(in.Prefix, prefixMarker)); err == nil && json.Unmarshal(recorded, &have) == nil && have == want {
		return nil
	}
	if err := os.MkdirAll(in.Prefix, 0o755); err != nil {
		return err
	}

	tree := backendTrees[in.Backend]
	in.Progress(fmt.Sprintf("preparing the Windows environment at %s", in.Prefix))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	boot := exec.CommandContext(ctx, in.Runtime.Wine(tree), "wineboot", "-u")
	boot.Env = in.Env
	boot.Stdout = in.Output
	boot.Stderr = in.Output
	if err := boot.Run(); err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "wineboot failed: %s", err)
	}
	wait := exec.CommandContext(ctx, in.Runtime.Wineserver(tree), "-w")
	wait.Env = in.Env
	if err := wait.Run(); err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "waiting for wineboot to finish: %s", err)
	}
	if err := makeUserFoldersPrivate(in.Prefix); err != nil {
		return err
	}
	if in.Backend == fixes.BackendDXVK {
		if err := installDXVK(in.Runtime, in.Prefix); err != nil {
			return err
		}
	}

	recorded, err := json.Marshal(want)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(in.Prefix, prefixMarker), recorded, 0o644)
}

func installDXVK(runtime wineruntime.Installed, prefix string) error {
	windows := filepath.Join(prefix, "drive_c", "windows")
	folders := map[wineruntime.DXVKBits]string{wineruntime.DXVK64: "system32", wineruntime.DXVK32: "syswow64"}
	for bits, folder := range folders {
		target := filepath.Join(windows, folder)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		dlls, err := filepath.Glob(filepath.Join(runtime.DXVK(bits), "*.dll"))
		if err != nil {
			return err
		}
		for _, dll := range dlls {
			content, err := os.ReadFile(dll)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(target, filepath.Base(dll)), content, 0o644); err != nil {
				return failure.Errorf(failure.CodePrefixFailed, "putting DXVK's %s into %s: %s", filepath.Base(dll), folder, err)
			}
		}
	}
	return nil
}

func makeUserFoldersPrivate(prefix string) error {
	homes, err := filepath.Glob(filepath.Join(prefix, "drive_c", "users", "*"))
	if err != nil {
		return err
	}
	for _, home := range homes {
		entries, err := os.ReadDir(home)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink == 0 {
				continue
			}
			link := filepath.Join(home, entry.Name())
			target, err := filepath.EvalSymlinks(link)
			if err == nil && strings.HasPrefix(target, prefix+string(filepath.Separator)) {
				continue
			}
			if err := os.Remove(link); err != nil {
				return failure.Errorf(failure.CodePrefixFailed, "unlinking %s from the Mac's folders: %s", link, err)
			}
			if err := os.Mkdir(link, 0o755); err != nil {
				return err
			}
		}
	}
	return nil
}
