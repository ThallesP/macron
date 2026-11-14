package wineruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/ThallesP/macron/internal/failure"
)

type Tree string

const (
	TreePlain Tree = "wine"
	TreeDXMT  Tree = "wine-dxmt"
)

type Installed struct {
	Version string
	Dir     string
}

func (r Installed) Root(tree Tree) string {
	return filepath.Join(r.Dir, string(tree))
}

func (r Installed) Wine(tree Tree) string {
	return filepath.Join(r.Root(tree), "bin", "wine")
}

func (r Installed) Wineserver(tree Tree) string {
	return filepath.Join(r.Root(tree), "bin", "wineserver")
}

const installedMarker = "installed.json"

var dxmtArchitectures = []string{"x86_64-windows", "x86_64-unix", "i386-windows"}

func Find(layout Layout, manifest Manifest) (Installed, error) {
	dir := layout.RuntimeDir(manifest.Version)
	recorded, err := os.ReadFile(filepath.Join(dir, installedMarker))
	if err != nil {
		return Installed{}, failure.Errorf(failure.CodeRuntimeMissing, "runtime %s is not installed; run macron runtime install", manifest.Version)
	}
	var found Manifest
	if err := json.Unmarshal(recorded, &found); err != nil || found != manifest {
		return Installed{}, failure.Errorf(failure.CodeRuntimeMissing, "runtime %s at %s is incomplete or different; run macron runtime install", manifest.Version, dir)
	}
	return Installed{Version: manifest.Version, Dir: dir}, nil
}

func Install(ctx context.Context, layout Layout, manifest Manifest, progress func(string)) (Installed, error) {
	if installed, err := Find(layout, manifest); err == nil {
		return installed, nil
	}

	progress(fmt.Sprintf("downloading %s", manifest.Wine.Name))
	wineArchive, err := Download(ctx, layout.DownloadsDir(), manifest.Wine)
	if err != nil {
		return Installed{}, err
	}
	progress(fmt.Sprintf("downloading %s", manifest.DXMT.Name))
	dxmtArchive, err := Download(ctx, layout.DownloadsDir(), manifest.DXMT)
	if err != nil {
		return Installed{}, err
	}

	dir := layout.RuntimeDir(manifest.Version)
	staging := dir + ".partial"
	if err := os.RemoveAll(staging); err != nil {
		return Installed{}, err
	}
	defer os.RemoveAll(staging)

	progress("unpacking Wine")
	if err := unpackWine(ctx, wineArchive, staging); err != nil {
		return Installed{}, err
	}
	progress("adding DXMT")
	if err := addDXMT(ctx, dxmtArchive, staging); err != nil {
		return Installed{}, err
	}

	recorded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Installed{}, err
	}
	if err := os.WriteFile(filepath.Join(staging, installedMarker), recorded, 0o644); err != nil {
		return Installed{}, err
	}
	if err := os.RemoveAll(dir); err != nil {
		return Installed{}, err
	}
	if err := os.Rename(staging, dir); err != nil {
		return Installed{}, err
	}
	return Installed{Version: manifest.Version, Dir: dir}, nil
}

func unpackWine(ctx context.Context, archive, staging string) error {
	unpacked := filepath.Join(staging, "unpacked-wine")
	if err := extract(ctx, archive, unpacked); err != nil {
		return err
	}
	roots, err := filepath.Glob(filepath.Join(unpacked, "*.app", "Contents", "Resources", "wine"))
	if err != nil || len(roots) != 1 {
		return failure.Errorf(failure.CodeExtractFailed, "%s does not hold exactly one Wine app", filepath.Base(archive))
	}
	if err := os.Rename(roots[0], filepath.Join(staging, string(TreePlain))); err != nil {
		return err
	}
	return os.RemoveAll(unpacked)
}

func addDXMT(ctx context.Context, archive, staging string) error {
	unpacked := filepath.Join(staging, "unpacked-dxmt")
	if err := extract(ctx, archive, unpacked); err != nil {
		return err
	}
	plain := filepath.Join(staging, string(TreePlain))
	dxmt := filepath.Join(staging, string(TreeDXMT))
	if err := cloneTree(ctx, plain, dxmt); err != nil {
		return err
	}

	for _, arch := range dxmtArchitectures {
		source, err := findDir(unpacked, arch)
		if err != nil {
			return err
		}
		if err := copyFiles(ctx, source, filepath.Join(dxmt, "lib", "wine", arch)); err != nil {
			return err
		}
	}
	return os.RemoveAll(unpacked)
}

func extract(ctx context.Context, archive, into string) error {
	if err := os.MkdirAll(into, 0o755); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "tar", "-xf", archive, "-C", into).CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "unpacking %s: %s: %s", filepath.Base(archive), err, output)
	}
	return nil
}

func cloneTree(ctx context.Context, from, to string) error {
	args := []string{"-R", from, to}
	if runtime.GOOS == "darwin" {
		args = append([]string{"-c"}, args...)
	}
	output, err := exec.CommandContext(ctx, "cp", args...).CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "copying the Wine tree: %s: %s", err, output)
	}
	return nil
}

func copyFiles(ctx context.Context, from, to string) error {
	if _, err := os.Stat(to); err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "Wine has no %s folder for DXMT's files", filepath.Base(to))
	}
	output, err := exec.CommandContext(ctx, "cp", "-R", from+"/.", to).CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "copying DXMT into Wine: %s: %s", err, output)
	}
	return nil
}

func findDir(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || found != "" {
			return err
		}
		if entry.IsDir() && entry.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", failure.Errorf(failure.CodeExtractFailed, "DXMT's archive has no %s folder", name)
	}
	return found, nil
}
