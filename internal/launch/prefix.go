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

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

type prefixRecord struct {
	Runtime string           `json:"runtime"`
	Tree    wineruntime.Tree `json:"tree"`
}

const prefixMarker = ".macron-prefix.json"

func EnsurePrefix(ctx context.Context, runtime wineruntime.Installed, tree wineruntime.Tree, prefix string, env []string, output io.Writer, progress func(string)) error {
	want := prefixRecord{Runtime: runtime.Version, Tree: tree}
	var have prefixRecord
	if recorded, err := os.ReadFile(filepath.Join(prefix, prefixMarker)); err == nil && json.Unmarshal(recorded, &have) == nil && have == want {
		return nil
	}
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return err
	}

	progress(fmt.Sprintf("preparing the Windows environment at %s", prefix))
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	boot := exec.CommandContext(ctx, runtime.Wine(tree), "wineboot", "-u")
	boot.Env = env
	boot.Stdout = output
	boot.Stderr = output
	if err := boot.Run(); err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "wineboot failed: %s", err)
	}
	wait := exec.CommandContext(ctx, runtime.Wineserver(tree), "-w")
	wait.Env = env
	if err := wait.Run(); err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "waiting for wineboot to finish: %s", err)
	}
	if err := makeUserFoldersPrivate(prefix); err != nil {
		return err
	}

	recorded, err := json.Marshal(want)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(prefix, prefixMarker), recorded, 0o644)
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
