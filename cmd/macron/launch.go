package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/launch"
	"github.com/ThallesP/macron/internal/wineruntime"
)

func launchCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("launch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	gameID := flags.String("game", "", "profile id from the fixes database")
	cwd := flags.String("cwd", "", "folder to start the game in (default: the exe's folder)")
	if err := flags.Parse(args); err != nil || *gameID == "" || flags.NArg() == 0 {
		fmt.Fprint(stderr, "usage: macron launch --game <id> [--cwd <dir>] -- <exe> [args]\n")
		return exitUsage
	}

	db, err := fixes.Load()
	if err != nil {
		return fail(stderr, err)
	}
	profile, err := db.Get(*gameID)
	if err != nil {
		return fail(stderr, err)
	}
	return startGame(ctx, gameStart{Profile: profile, Exe: flags.Arg(0), Cwd: *cwd, Args: flags.Args()[1:], WaitForWine: true}, stderr)
}

func killCommand(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	gameID := flags.String("game", "", "profile id whose Windows environment to stop")
	if err := flags.Parse(args); err != nil || *gameID == "" {
		fmt.Fprint(stderr, "usage: macron kill --game <id>\n")
		return exitUsage
	}

	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}
	installed, err := wineruntime.Find(layout, wineruntime.Pinned)
	if err != nil {
		return fail(stderr, err)
	}
	if err := launch.Kill(ctx, installed, layout.PrefixDir(*gameID)); err != nil {
		return fail(stderr, err)
	}
	return 0
}

type gameStart struct {
	Profile     fixes.Profile
	Prefix      string
	Exe         string
	Cwd         string
	Args        []string
	WaitForWine bool
	SteamBridge bool
}

func startGame(ctx context.Context, in gameStart, stderr io.Writer) int {
	layout, err := wineruntime.DefaultLayout()
	if err != nil {
		return fail(stderr, err)
	}
	prefix := in.Prefix
	if prefix == "" {
		prefix = layout.PrefixDir(in.Profile.ID)
	}
	logDir := filepath.Join(layout.LogsDir(), in.Profile.ID)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fail(stderr, err)
	}
	logFile, err := os.Create(filepath.Join(logDir, time.Now().UTC().Format("20060102T150405Z")+".log"))
	if err != nil {
		return fail(stderr, err)
	}
	defer logFile.Close()
	output := io.MultiWriter(stderr, logFile)
	progress := timedProgressTo(output, time.Now())
	progress(fmt.Sprintf("game %s, log %s", in.Profile.ID, logFile.Name()))

	openGL := os.Getenv("MACRON_OPENGL")
	if openGL != "" && openGL != "llvmpipe" && openGL != "apple-software" {
		return failTo(progress, failure.Errorf(failure.CodeInvalidInput, "MACRON_OPENGL=%q; the choices are llvmpipe and apple-software", openGL))
	}
	profile, err := withBackendFromEnv(in.Profile, os.Getenv("MACRON_BACKEND"))
	if err != nil {
		return failTo(progress, err)
	}
	if _, err := os.Stat(layout.ShowFPSFile()); err == nil {
		profile = withFrameCounter(profile)
		progress(fmt.Sprintf("logging the frame rate (%s exists)", layout.ShowFPSFile()))
	}
	if _, err := os.Stat(layout.DLSSFile()); err == nil && profile.Backend == fixes.BackendDXMT {
		profile = withDLSS(profile)
		progress(fmt.Sprintf("DXMT answers as an NVIDIA GPU, so DLSS runs on MetalFX (%s exists)", layout.DLSSFile()))
	}
	if openGL == "apple-software" {
		profile = withAppleSoftwareRenderer(profile)
		progress("OpenGL may use Apple's software renderer when there is no hardware one (MACRON_OPENGL=apple-software)")
	}
	if profile.Backend != in.Profile.Backend {
		progress(fmt.Sprintf("backend %s instead of the profile's %s (MACRON_BACKEND)", profile.Backend, in.Profile.Backend))
	}
	installed, err := wineruntime.Install(ctx, layout, wineruntime.Pinned, progress)
	if err != nil {
		return failTo(progress, err)
	}
	if err := followGameModeSwitch(ctx, layout, installed, progress); err != nil {
		return failTo(progress, err)
	}
	exitCode, err := launch.Run(ctx, launch.Input{
		Runtime:        installed,
		Installers:     fixes.Installers,
		DownloadsDir:   layout.DownloadsDir(),
		Profile:        profile,
		Prefix:         prefix,
		Exe:            in.Exe,
		Cwd:            in.Cwd,
		Args:           in.Args,
		Output:         output,
		Progress:       progress,
		WaitForWine:    in.WaitForWine,
		SoftwareOpenGL: openGL == "llvmpipe",
		SteamBridge:    in.SteamBridge,
		Chip:           hostChip(ctx),
	})
	if err != nil {
		return failTo(progress, err)
	}
	return exitCode
}

// The runtime's Wine loader runs from a games-category app bundle while
// the game-mode file exists, and as Wine ships it otherwise.
func followGameModeSwitch(ctx context.Context, layout wineruntime.Layout, installed wineruntime.Installed, progress func(string)) error {
	_, err := os.Stat(layout.GameModeFile())
	on := err == nil
	for _, tree := range []wineruntime.Tree{wineruntime.TreeDXMT, wineruntime.TreePlain} {
		root := installed.Root(tree)
		if on == wineruntime.GameModeApplied(root) {
			continue
		}
		if on {
			if err := wineruntime.EnsureGameMode(ctx, root); err != nil {
				return err
			}
		} else if err := wineruntime.RevertGameMode(root); err != nil {
			return err
		}
	}
	if on {
		progress(fmt.Sprintf("Wine runs as a game app, so macOS Game Mode can turn on (%s exists)", layout.GameModeFile()))
	}
	return nil
}

func withBackendFromEnv(profile fixes.Profile, value string) (fixes.Profile, error) {
	if value == "" {
		return profile, nil
	}
	backend := fixes.Backend(value)
	if !backend.IsKnown() {
		return fixes.Profile{}, failure.Errorf(failure.CodeInvalidInput, "MACRON_BACKEND=%q; the choices are dxmt, dxvk and wined3d", value)
	}
	profile.Backend = backend
	return profile, nil
}

func withFrameCounter(profile fixes.Profile) fixes.Profile {
	env := maps.Clone(profile.Env)
	if env == nil {
		env = map[string]string{}
	}
	env["WINEDEBUG"] = "-all,+fps"
	env["DXVK_HUD"] = "fps"
	env["MTL_HUD_ENABLED"] = "1"
	env["MTL_HUD_LOG_ENABLED"] = "1"
	env["DXMT_CONFIG"] = strings.TrimPrefix(env["DXMT_CONFIG"]+";dxmt.logLatency=True", ";")
	profile.Env = env
	return profile
}

func withDLSS(profile fixes.Profile) fixes.Profile {
	env := maps.Clone(profile.Env)
	if env == nil {
		env = map[string]string{}
	}
	env["DXMT_ENABLE_NVEXT"] = "1"
	env["WINE_D3D_CONFIG"] = strings.TrimPrefix(env["WINE_D3D_CONFIG"]+",VideoPciVendorID=0x10de,VideoPciDeviceID=0x2684", ",")
	profile.Env = env
	return profile
}

func withAppleSoftwareRenderer(profile fixes.Profile) fixes.Profile {
	profile.Registry = append(slices.Clone(profile.Registry), fixes.RegistryValue{
		Key:   `HKEY_CURRENT_USER\Software\Wine\Mac Driver`,
		Name:  "AllowSoftwareRendering",
		Type:  "sz",
		Value: "y",
	})
	return profile
}

func progressTo(w io.Writer) func(string) {
	return func(message string) {
		fmt.Fprintf(w, "macron: %s\n", message)
	}
}

// timedProgressTo starts each line of a game's log with the wall time (UTC,
// like the log's name) and the seconds since macron started the game, so a
// slow start shows where its time went.
func timedProgressTo(w io.Writer, start time.Time) func(string) {
	return func(message string) {
		now := time.Now()
		fmt.Fprintf(w, "macron: %s +%.2fs %s\n", now.UTC().Format("15:04:05.000Z"), now.Sub(start).Seconds(), message)
	}
}

func fail(w io.Writer, err error) int {
	return failTo(progressTo(w), err)
}

func failTo(progress func(string), err error) int {
	code := failure.CodeOf(err)
	if code == "" {
		code = "UNEXPECTED"
	}
	progress(fmt.Sprintf("%s: %s", code, err))
	return exitFailed
}

// hostChip is the Mac's chip ("Apple M3"), or nothing on a Mac that is not
// Apple silicon. Asked through the arm64 sysctl: an Intel one under Rosetta
// names a virtual CPU.
func hostChip(ctx context.Context) string {
	chip, err := wineruntime.HostCommand(ctx, "arch", "-arm64", "sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(chip))
}
