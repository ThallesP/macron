package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
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

const (
	prefixMarker    = ".macron-prefix.json"
	templatesFolder = "prefix-templates"
	// wineboot keeps the mtime of the wine.inf it last installed here and
	// installs it again when the two differ (programs/wineboot/wineboot.c,
	// update_timestamp).
	updateTimestamp = ".update-timestamp"
	wineInf         = "share/wine/wine.inf"
	registryFile    = "system.reg"
)

// A new game's prefix is a clone of a prefix made once per runtime and Wine
// tree. A prefix of an earlier runtime is only updated by wineboot when the
// runtime's wine.inf differs from the one wineboot installed into it: Wine
// loads its builtin modules from the runtime, not from the prefix, so the
// prefix's copies of them do not need refreshing.
func EnsurePrefix(ctx context.Context, in PrefixInput) error {
	want := prefixRecord{Runtime: in.Runtime.Version, Backend: in.Backend}
	have, hasRecord := readPrefixRecord(in.Prefix)
	if hasRecord && have == want {
		return nil
	}

	tree := backendTrees[in.Backend]
	switch {
	case hasRecord && have.Backend == want.Backend && wineInfInstalled(in.Prefix, in.Runtime.Root(tree)):
		in.Progress(fmt.Sprintf("the Windows environment at %s needs no update for this runtime", in.Prefix))
	case isEmptyOrMissing(in.Prefix):
		template, err := ensureTemplate(ctx, in, tree)
		if err != nil {
			return err
		}
		if err := clonePrefix(ctx, template, in.Prefix); err != nil {
			return err
		}
		in.Progress(fmt.Sprintf("made the Windows environment at %s from the runtime's template", in.Prefix))
	default:
		if err := os.MkdirAll(in.Prefix, 0o755); err != nil {
			return err
		}
		in.Progress(fmt.Sprintf("preparing the Windows environment at %s", in.Prefix))
		if err := bootPrefix(ctx, in.Runtime, tree, in.Env, in.Output); err != nil {
			return err
		}
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

func readPrefixRecord(prefix string) (prefixRecord, bool) {
	var record prefixRecord
	recorded, err := os.ReadFile(filepath.Join(prefix, prefixMarker))
	if err != nil || json.Unmarshal(recorded, &record) != nil {
		return prefixRecord{}, false
	}
	return record, true
}

func wineInfInstalled(prefix, root string) bool {
	installed, err := os.ReadFile(filepath.Join(prefix, updateTimestamp))
	if err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(root, wineInf))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(installed)) == strconv.FormatInt(info.ModTime().Unix(), 10)
}

func isEmptyOrMissing(dir string) bool {
	entries, err := os.ReadDir(dir)
	return errors.Is(err, fs.ErrNotExist) || (err == nil && len(entries) == 0)
}

// bootPrefix runs wineboot -u in WINEPREFIX and waits until the Wine server
// has written the registry to disk. The server is started with no linger,
// so the wait ends when wineboot is done instead of three seconds later.
func bootPrefix(ctx context.Context, runtime wineruntime.Installed, tree wineruntime.Tree, env []string, output io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	startServerWithoutLinger(ctx, runtime.Wineserver(tree), env)
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
	return nil
}

// startServerWithoutLinger starts the prefix's Wine server with -p0: it then
// exits as soon as its last program does, where one started by Wine stays
// three seconds (server/main.c, master_socket_timeout) and wineserver -w
// waits them out. When a server already runs for the prefix it stays as it is.
func startServerWithoutLinger(ctx context.Context, wineserver string, env []string) {
	server := exec.CommandContext(ctx, wineserver, "-p0")
	server.Env = env
	server.Run()
}

// ensureTemplate makes the tree's template prefix once per runtime, in a
// folder of its own that is renamed into place when complete, so a game
// started meanwhile never clones half of one.
func ensureTemplate(ctx context.Context, in PrefixInput, tree wineruntime.Tree) (string, error) {
	template := filepath.Join(in.Runtime.Dir, templatesFolder, string(tree))
	if _, err := os.Stat(filepath.Join(template, registryFile)); err == nil {
		return template, nil
	}
	if err := os.MkdirAll(filepath.Dir(template), 0o755); err != nil {
		return "", err
	}
	partial, err := os.MkdirTemp(filepath.Dir(template), string(tree)+".partial-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(partial)

	in.Progress("preparing the Windows environment new games start from (once per runtime)")
	env := slices.Concat(in.Env, []string{"WINEPREFIX=" + partial})
	if err := bootPrefix(ctx, in.Runtime, tree, env, in.Output); err != nil {
		return "", err
	}
	if err := makeUserFoldersPrivate(partial); err != nil {
		return "", err
	}
	if err := os.Rename(partial, template); err != nil {
		if _, statErr := os.Stat(filepath.Join(template, registryFile)); statErr == nil {
			return template, nil
		}
		return "", err
	}
	return template, nil
}

// clonePrefix copies the template with APFS clones (cp -c), which share the
// template's blocks until either side changes, and with a plain copy where
// cloning fails (another volume, another file system).
func clonePrefix(ctx context.Context, template, prefix string) error {
	if err := os.Remove(prefix); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(prefix), 0o755); err != nil {
		return err
	}
	if exec.CommandContext(ctx, "cp", "-c", "-R", template, prefix).Run() == nil {
		return nil
	}
	if err := os.RemoveAll(prefix); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "cp", "-R", template, prefix).CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "copying the template prefix to %s: %s: %s", prefix, err, output)
	}
	return nil
}

func installDXVK(runtime wineruntime.Installed, prefix string) error {
	return copyIntoWindows(prefix, map[string]string{
		"system32": runtime.DXVK(wineruntime.DXVK64),
		"syswow64": runtime.DXVK(wineruntime.DXVK32),
	})
}

func installMesa(runtime wineruntime.Installed, prefix string) error {
	return copyIntoWindows(prefix, map[string]string{
		"system32": runtime.Mesa(wineruntime.Mesa64),
		"syswow64": runtime.Mesa(wineruntime.Mesa32),
	})
}

func copyIntoWindows(prefix string, sources map[string]string) error {
	windows := filepath.Join(prefix, "drive_c", "windows")
	for folder, source := range sources {
		target := filepath.Join(windows, folder)
		if _, err := os.Stat(target); err != nil {
			continue
		}
		dlls, err := filepath.Glob(filepath.Join(source, "*.dll"))
		if err != nil {
			return err
		}
		for _, dll := range dlls {
			content, err := os.ReadFile(dll)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(target, filepath.Base(dll)), content, 0o644); err != nil {
				return failure.Errorf(failure.CodePrefixFailed, "putting %s into %s: %s", filepath.Base(dll), folder, err)
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

// PrepareTemplates makes the template prefix of each Wine tree, so that the
// first game of a runtime starts from a clone too.
func PrepareTemplates(ctx context.Context, runtime wineruntime.Installed, output io.Writer, progress func(string)) error {
	for _, backend := range []fixes.Backend{fixes.BackendDXMT, fixes.BackendWineD3D} {
		profile := fixes.Default("template")
		profile.Backend = backend
		in := PrefixInput{Runtime: runtime, Backend: backend, Env: Environment(profile, "", os.Environ()), Output: output, Progress: progress}
		if _, err := ensureTemplate(ctx, in, backendTrees[backend]); err != nil {
			return err
		}
	}
	return nil
}
