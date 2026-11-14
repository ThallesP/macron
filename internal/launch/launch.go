package launch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

type Input struct {
	Runtime        wineruntime.Installed
	Installers     map[string]fixes.Installer
	DownloadsDir   string
	Profile        fixes.Profile
	Prefix         string
	Exe            string
	Cwd            string
	Args           []string
	Output         io.Writer
	Progress       func(string)
	WaitForWine    bool
	SoftwareOpenGL bool
}

var backendTrees = map[fixes.Backend]wineruntime.Tree{
	fixes.BackendDXMT:    wineruntime.TreeDXMT,
	fixes.BackendDXVK:    wineruntime.TreePlain,
	fixes.BackendWineD3D: wineruntime.TreePlain,
}

var backendDLLOverrides = map[fixes.Backend]map[string]string{
	fixes.BackendDXMT:    {},
	fixes.BackendDXVK:    {"d3d11": "n", "d3d10core": "n", "dxgi": "n"},
	fixes.BackendWineD3D: {},
}

var alwaysDisabledDLLs = map[string]string{"mscoree": "", "mshtml": ""}

func Run(ctx context.Context, in Input) (int, error) {
	exe, err := filepath.Abs(in.Exe)
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(exe); err != nil {
		return 0, failure.Errorf(failure.CodeLaunchFailed, "%s does not exist", exe)
	}
	tree := backendTrees[in.Profile.Backend]
	env := Environment(in.Profile, in.Prefix, os.Environ())
	if in.SoftwareOpenGL {
		env = append(env, "GALLIUM_DRIVER=llvmpipe", "WINEDLLOVERRIDES="+dllOverrides(in.Profile, map[string]string{"opengl32": "n,b"}))
	}

	if err := EnsurePrefix(ctx, PrefixInput{Runtime: in.Runtime, Backend: in.Profile.Backend, Prefix: in.Prefix, Env: env, Output: in.Output, Progress: in.Progress}); err != nil {
		return 0, err
	}
	if in.SoftwareOpenGL {
		if err := wineruntime.EnsureMesa(ctx, in.DownloadsDir, in.Runtime, wineruntime.Mesa, in.Progress); err != nil {
			return 0, err
		}
		if err := installMesa(in.Runtime, in.Prefix); err != nil {
			return 0, err
		}
		in.Progress("OpenGL is drawn in software by Mesa llvmpipe (MACRON_OPENGL=llvmpipe)")
	}
	if err := EnsureRegistry(ctx, in, env); err != nil {
		return 0, err
	}
	if err := EnsureInstalls(ctx, in, env); err != nil {
		return 0, err
	}
	if err := WriteFiles(in.Profile.Files, Places(in.Prefix, filepath.Dir(exe))); err != nil {
		return 0, err
	}

	in.Progress(fmt.Sprintf("starting %s with the %s backend", filepath.Base(exe), in.Profile.Backend))
	game := exec.CommandContext(ctx, in.Runtime.Wine(tree), slices.Concat([]string{exe}, in.Args, in.Profile.Args)...)
	game.Dir = in.Cwd
	if game.Dir == "" {
		game.Dir = filepath.Dir(exe)
	}
	game.Env = env
	game.Stdout = in.Output
	game.Stderr = in.Output
	exitCode, err := exitCodeOf(game.Run())
	if err != nil {
		return 0, failure.Errorf(failure.CodeLaunchFailed, "starting Wine: %s", err)
	}
	in.Progress(fmt.Sprintf("%s exited with %d", filepath.Base(exe), exitCode))

	if in.WaitForWine {
		waitForWineserver(ctx, in.Runtime.Wineserver(tree), env)
	}
	return exitCode, nil
}

func Environment(profile fixes.Profile, prefix string, base []string) []string {
	env := slices.Concat(base, []string{
		"WINEPREFIX=" + prefix,
		"WINEDEBUG=-all",
		"WINEDLLOVERRIDES=" + dllOverrides(profile, nil),
	})
	for _, name := range slices.Sorted(maps.Keys(profile.Env)) {
		env = append(env, name+"="+profile.Env[name])
	}
	return env
}

func dllOverrides(profile fixes.Profile, extra map[string]string) string {
	overrides := maps.Clone(alwaysDisabledDLLs)
	maps.Copy(overrides, backendDLLOverrides[profile.Backend])
	maps.Copy(overrides, extra)
	maps.Copy(overrides, profile.DLLOverrides)
	var entries []string
	for _, dll := range slices.Sorted(maps.Keys(overrides)) {
		entries = append(entries, dll+"="+overrides[dll])
	}
	return strings.Join(entries, ";")
}

func Kill(ctx context.Context, runtime wineruntime.Installed, prefix string) error {
	kill := exec.CommandContext(ctx, runtime.Wineserver(wineruntime.TreePlain), "-k")
	kill.Env = append(os.Environ(), "WINEPREFIX="+prefix)
	if _, err := exitCodeOf(kill.Run()); err != nil {
		return failure.Errorf(failure.CodeLaunchFailed, "stopping Wine: %s", err)
	}
	return nil
}

func waitForWineserver(ctx context.Context, wineserver string, env []string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	wait := exec.CommandContext(ctx, wineserver, "-w")
	wait.Env = env
	wait.Run()
}

func exitCodeOf(err error) (int, error) {
	var exited *exec.ExitError
	if errors.As(err, &exited) && exited.ExitCode() >= 0 {
		return exited.ExitCode(), nil
	}
	return 0, err
}
