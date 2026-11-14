package check

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/scenario"
	"github.com/ThallesP/macron/internal/wineruntime"
)

type Game struct {
	Exe string
	Cwd string
}

func PrepareGame(ctx context.Context, s scenario.Scenario, cacheDir, workDir string) (Game, error) {
	root, err := filepath.Abs(s.Dir)
	if err != nil {
		return Game{}, err
	}
	if len(s.Downloads) > 0 {
		root, err = filepath.Abs(filepath.Join(workDir, "game"))
		if err != nil {
			return Game{}, err
		}
		if err := os.RemoveAll(root); err != nil {
			return Game{}, err
		}
	}
	for _, d := range s.Downloads {
		archive, err := wineruntime.Download(ctx, cacheDir, d.Archive)
		if err != nil {
			return Game{}, err
		}
		into := filepath.Join(root, filepath.FromSlash(d.Into))
		if err := os.MkdirAll(into, 0o755); err != nil {
			return Game{}, err
		}
		if err := unpack(ctx, d.Unpack, archive, into); err != nil {
			return Game{}, err
		}
	}

	game := Game{Exe: filepath.Join(root, filepath.FromSlash(s.Exe)), Cwd: filepath.Join(root, filepath.FromSlash(s.Cwd))}
	if s.Cwd == "" {
		game.Cwd = filepath.Dir(game.Exe)
	}
	if _, err := os.Stat(game.Exe); err != nil {
		if s.Build != "" {
			return Game{}, failure.Errorf(failure.CodeInvalidInput, "%s is not built; it comes from %s", game.Exe, s.Build)
		}
		return Game{}, failure.Errorf(failure.CodeExtractFailed, "the downloads unpacked, but %s is not in them", s.Exe)
	}
	return game, nil
}

func WorkDir(s scenario.Scenario) string {
	return filepath.Join(os.TempDir(), "macron-check", s.Name)
}

func unpack(ctx context.Context, format, archive, into string) error {
	if format == "none" {
		content, err := os.ReadFile(archive)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(into, filepath.Base(archive)), content, 0o755)
	}
	command := exec.CommandContext(ctx, "tar", "-xf", archive, "-C", into)
	if format == "7z" && runtime.GOOS == "windows" {
		command = exec.CommandContext(ctx, "7z", "x", "-y", "-o"+into, archive)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "unpacking %s: %s: %s", filepath.Base(archive), err, tail(output))
	}
	return nil
}

func tail(output []byte) string {
	if len(output) > 2000 {
		return fmt.Sprintf("...%s", output[len(output)-2000:])
	}
	return string(output)
}
