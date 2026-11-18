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
	SteamBridge    bool
}

var backendTrees = map[fixes.Backend]wineruntime.Tree{
	fixes.BackendDXMT:    wineruntime.TreeDXMT,
	fixes.BackendDXVK:    wineruntime.TreePlain,
	fixes.BackendWineD3D: wineruntime.TreePlain,
}

var backendDLLOverrides = map[fixes.Backend]map[string]string{
	fixes.BackendDXMT:    {},
	fixes.BackendDXVK:    {"d3d9": "n", "d3d11": "n", "d3d10core": "n", "dxgi": "n"},
	fixes.BackendWineD3D: {},
}

// winemenubuilder.exe would make Mac menu entries and file associations for
// what games install, under Wine's name; it starts with every cold start.
var alwaysDisabledDLLs = map[string]string{"mscoree": "", "mshtml": "", "winedbg.exe": "", "winemenubuilder.exe": ""}

const ExitStopped = 130

// When ctx is cancelled (Steam's Stop button reaches macron as SIGINT or
// SIGTERM), the Wine server is told to end every program first: the game is
// not the process macron started, and it holds macron's output pipe open.
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
	if inserted := insertedLibraries(os.Getenv("DYLD_INSERT_LIBRARIES")); inserted != "" {
		in.Progress("the caller's DYLD_INSERT_LIBRARIES goes on to Wine: " + inserted)
	}
	if in.SoftwareOpenGL {
		env = append(env, "GALLIUM_DRIVER=llvmpipe", "WINEDLLOVERRIDES="+dllOverrides(in.Profile, map[string]string{"opengl32": "n,b"}))
	}

	if err := EnsurePrefix(ctx, PrefixInput{Runtime: in.Runtime, Backend: in.Profile.Backend, Prefix: in.Prefix, Env: env, Output: in.Output, Progress: in.Progress}); err != nil {
		return 0, err
	}
	if err := syncNGXCore(in.Runtime.Root(tree), in.Prefix); err != nil {
		return 0, failure.Errorf(failure.CodePrefixFailed, "putting the runtime's nvngx.dll into the prefix: %s", err)
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
	in.Progress("prefix ready")

	in.Progress(fmt.Sprintf("starting %s with the %s backend", filepath.Base(exe), in.Profile.Backend))
	command := slices.Concat([]string{exe}, in.Args, in.Profile.Args)
	if in.SteamBridge {
		env, command, err = withSteamBridge(ctx, in, tree, env, command)
		if err != nil {
			return 0, err
		}
		in.Progress("Steam bridge ready")
	}
	game := exec.CommandContext(ctx, in.Runtime.Wine(tree), command...)
	game.Dir = in.Cwd
	if game.Dir == "" {
		game.Dir = filepath.Dir(exe)
	}
	game.Env = env
	game.Stdout = in.Output
	game.Stderr = in.Output
	game.Cancel = func() error {
		in.Progress("asked to stop: ending every program in this Windows environment")
		stopping, release := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer release()
		Kill(stopping, in.Runtime, in.Prefix)
		return game.Process.Kill()
	}
	game.WaitDelay = 10 * time.Second
	if err := game.Start(); err != nil {
		return 0, failure.Errorf(failure.CodeLaunchFailed, "starting Wine: %s", err)
	}
	in.Progress(fmt.Sprintf("game process started (Wine process %d)", game.Process.Pid))
	exitCode, err := exitCodeOf(game.Wait())
	if ctx.Err() != nil {
		return ExitStopped, nil
	}
	if err != nil {
		return 0, failure.Errorf(failure.CodeLaunchFailed, "starting Wine: %s", err)
	}
	in.Progress(fmt.Sprintf("%s exited with %d", filepath.Base(exe), exitCode))

	if in.WaitForWine {
		waitForWineserver(ctx, in.Runtime.Wineserver(tree), env)
	}
	return exitCode, nil
}

// insertedLibraries names the libraries of a DYLD_INSERT_LIBRARIES value
// without their folders, which hold the user's name. Steam sets the variable
// to its overlay loader for every game it starts.
func insertedLibraries(value string) string {
	names := []string{}
	for _, library := range strings.Split(value, ":") {
		if library != "" {
			names = append(names, filepath.Base(library))
		}
	}
	return strings.Join(names, ", ")
}

func Environment(profile fixes.Profile, prefix string, base []string) []string {
	env := slices.Concat([]string{"WINEDEBUG=-all"}, base, []string{
		"WINEPREFIX=" + prefix,
		"WINEDLLOVERRIDES=" + dllOverrides(profile, nil),
	})
	for _, name := range slices.Sorted(maps.Keys(profile.Env)) {
		env = append(env, name+"="+profile.Env[name])
	}
	if both, ok := dxmtConfigOfBoth(profile, base); ok {
		env = append(env, "DXMT_CONFIG="+both)
	}
	return env
}

// DXMT_CONFIG is a list of options, and DXMT takes the last value of each.
// When the profile and the user both set it, the game gets the profile's
// options followed by the user's, so the user's win and the profile's others
// stay.
func dxmtConfigOfBoth(profile fixes.Profile, base []string) (string, bool) {
	own, ok := profile.Env["DXMT_CONFIG"]
	if !ok {
		return "", false
	}
	for _, entry := range slices.Backward(base) {
		if users, ok := strings.CutPrefix(entry, "DXMT_CONFIG="); ok {
			return own + ";" + users, true
		}
	}
	return "", false
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
	if errors.Is(err, exec.ErrWaitDelay) {
		return 0, nil
	}
	return 0, err
}
