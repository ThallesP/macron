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

func PrepareGame(ctx context.Context, s scenario.Scenario, cacheDir, workDir string) (string, error) {
	if s.Download == nil {
		exe := filepath.Join(s.Dir, s.Exe)
		if _, err := os.Stat(exe); err != nil {
			return "", failure.Errorf(failure.CodeInvalidInput, "%s is not built; it comes from %s", exe, s.Build)
		}
		return exe, nil
	}

	archive, err := wineruntime.Download(ctx, cacheDir, s.Download.Archive)
	if err != nil {
		return "", err
	}
	gameDir := filepath.Join(workDir, "game")
	if err := os.RemoveAll(gameDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return "", err
	}
	if err := unpack(ctx, s.Download.Unpack, archive, gameDir); err != nil {
		return "", err
	}
	exe := filepath.Join(gameDir, filepath.FromSlash(s.Exe))
	if _, err := os.Stat(exe); err != nil {
		return "", failure.Errorf(failure.CodeExtractFailed, "%s unpacked, but %s is not in it", s.Download.Name, s.Exe)
	}
	return exe, nil
}

func WorkDir(s scenario.Scenario) string {
	return filepath.Join(os.TempDir(), "macron-check", s.Name)
}

func unpack(ctx context.Context, format, archive, into string) error {
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
