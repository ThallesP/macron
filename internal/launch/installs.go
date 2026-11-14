package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

const installsMarker = ".macron-installs.json"

func EnsureInstalls(ctx context.Context, in Input, env []string) error {
	markerPath := filepath.Join(in.Prefix, installsMarker)
	done := []string{}
	if recorded, err := os.ReadFile(markerPath); err == nil {
		json.Unmarshal(recorded, &done)
	}

	tree := backendTrees[in.Profile.Backend]
	for _, name := range in.Profile.Installs {
		if slices.Contains(done, name) {
			continue
		}
		installer, ok := in.Installers[name]
		if !ok {
			return failure.Errorf(failure.CodeBadProfile, "unknown extra install %q", name)
		}
		in.Progress(fmt.Sprintf("installing %s into the Windows environment", name))
		for _, step := range installer.Steps {
			if err := runInstallStep(ctx, in, tree, env, step, installer.GoodExitCodes); err != nil {
				return err
			}
		}
		done = append(done, name)
		recorded, err := json.Marshal(done)
		if err != nil {
			return err
		}
		if err := os.WriteFile(markerPath, recorded, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func runInstallStep(ctx context.Context, in Input, tree wineruntime.Tree, env []string, step fixes.InstallStep, goodExitCodes []int) error {
	file, err := wineruntime.Download(ctx, in.DownloadsDir, step.Archive)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	install := exec.CommandContext(ctx, in.Runtime.Wine(tree), append([]string{file}, step.Args...)...)
	install.Env = env
	install.Stdout = in.Output
	install.Stderr = in.Output
	err = install.Run()
	var exited *exec.ExitError
	if err != nil && !(errors.As(err, &exited) && slices.Contains(goodExitCodes, exited.ExitCode())) {
		return failure.Errorf(failure.CodePrefixFailed, "%s failed: %s", step.Archive.Name, err)
	}
	waitForWineserver(ctx, in.Runtime.Wineserver(tree), env)
	return nil
}
