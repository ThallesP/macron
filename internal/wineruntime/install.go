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

func (r Installed) DXVK(bits DXVKBits) string {
	return filepath.Join(r.Dir, "dxvk", string(bits))
}

type DXVKBits string

const (
	DXVK64 DXVKBits = "x64"
	DXVK32 DXVKBits = "x32"
)

const installedMarker = "installed.json"

const winemacDriver = "lib/wine/x86_64-unix/winemac.so"

var dxmtArchitectures = []string{"x86_64-windows", "x86_64-unix", "i386-windows"}

var windowsArchitectures = []string{"i386-windows", "x86_64-windows"}

// dxmtOverlayModules are what a DXMT overlay must hold. They only work
// together: d3d11.dll calls winemetal.dll, which calls winemetal.so by number,
// and dxgi.dll trusts the certificate of the nvngx.dll it was built with.
var dxmtOverlayModules = []string{
	"lib/wine/x86_64-windows/d3d11.dll",
	"lib/wine/x86_64-windows/winemetal.dll",
	"lib/wine/x86_64-windows/dxgi.dll",
	"lib/wine/x86_64-windows/nvngx.dll",
	"lib/wine/i386-windows/d3d11.dll",
	"lib/wine/i386-windows/winemetal.dll",
	"lib/wine/i386-windows/dxgi.dll",
	"lib/wine/x86_64-unix/winemetal.so",
}

// overlayUnixLibs are the overlay's Unix libraries that go into the plain
// tree too, because the overlay's Windows modules there call into them:
// opengl32.dll's calls to its Unix side are numbered, and patch 0007 adds one.
var overlayUnixLibs = []string{"lib/wine/x86_64-unix/opengl32.so"}

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
	progress(fmt.Sprintf("downloading %s", manifest.WineOverlay.Name))
	overlayArchive, err := Download(ctx, layout.DownloadsDir(), manifest.WineOverlay)
	if err != nil {
		return Installed{}, err
	}
	progress(fmt.Sprintf("downloading %s", manifest.DXMT.Name))
	dxmtArchive, err := Download(ctx, layout.DownloadsDir(), manifest.DXMT)
	if err != nil {
		return Installed{}, err
	}
	progress(fmt.Sprintf("downloading %s", manifest.DXMTOverlay.Name))
	dxmtOverlayArchive, err := Download(ctx, layout.DownloadsDir(), manifest.DXMTOverlay)
	if err != nil {
		return Installed{}, err
	}
	progress(fmt.Sprintf("downloading %s", manifest.DXVK.Name))
	dxvkArchive, err := Download(ctx, layout.DownloadsDir(), manifest.DXVK)
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
	progress("adding Macron's DXMT patches")
	if err := addDXMTOverlay(ctx, dxmtOverlayArchive, staging); err != nil {
		return Installed{}, err
	}
	progress("adding Macron's Wine patches")
	if err := addWineOverlay(ctx, overlayArchive, staging); err != nil {
		return Installed{}, err
	}
	progress("adding DXVK")
	if err := addDXVK(ctx, dxvkArchive, staging); err != nil {
		return Installed{}, err
	}
	if err := addSteamBridge(ctx, layout, manifest.SteamBridge, staging, progress); err != nil {
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

// The DXMT overlay (runtime/dxmt) replaces DXMT's own modules in the DXMT tree
// only: the plain tree keeps Wine's d3d11.dll and has no winemetal.
func addDXMTOverlay(ctx context.Context, archive, staging string) error {
	unpacked := filepath.Join(staging, "unpacked-dxmt-overlay")
	if err := extract(ctx, archive, unpacked); err != nil {
		return err
	}
	for _, module := range dxmtOverlayModules {
		if _, err := os.Stat(filepath.Join(unpacked, module)); err != nil {
			return failure.Errorf(failure.CodeExtractFailed, "%s has no %s", filepath.Base(archive), module)
		}
	}
	if err := copyTree(ctx, unpacked, filepath.Join(staging, string(TreeDXMT))); err != nil {
		return err
	}
	return os.RemoveAll(unpacked)
}

func addWineOverlay(ctx context.Context, archive, staging string) error {
	unpacked := filepath.Join(staging, "unpacked-overlay")
	if err := extract(ctx, archive, unpacked); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(unpacked, winemacDriver)); err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "%s has no %s", filepath.Base(archive), winemacDriver)
	}
	dxmt := filepath.Join(staging, string(TreeDXMT))
	if _, err := os.Stat(filepath.Join(dxmt, winemacDriver)); err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "Wine has no %s to replace", winemacDriver)
	}
	if err := copyTree(ctx, unpacked, dxmt); err != nil {
		return err
	}
	for _, arch := range windowsArchitectures {
		modules := filepath.Join(unpacked, "lib", "wine", arch)
		if _, err := os.Stat(modules); err != nil {
			continue
		}
		if err := copyTree(ctx, modules, filepath.Join(staging, string(TreePlain), "lib", "wine", arch)); err != nil {
			return err
		}
	}
	for _, lib := range overlayUnixLibs {
		source := filepath.Join(unpacked, lib)
		if _, err := os.Stat(source); err != nil {
			continue
		}
		output, err := exec.CommandContext(ctx, "cp", source, filepath.Join(staging, string(TreePlain), lib)).CombinedOutput()
		if err != nil {
			return failure.Errorf(failure.CodeExtractFailed, "copying %s into the plain Wine tree: %s: %s", filepath.Base(lib), err, output)
		}
	}
	return os.RemoveAll(unpacked)
}

func copyTree(ctx context.Context, source, target string) error {
	output, err := exec.CommandContext(ctx, "cp", "-R", source+"/.", target).CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "copying %s into %s: %s: %s", filepath.Base(source), target, err, output)
	}
	return nil
}

func addDXVK(ctx context.Context, archive, staging string) error {
	unpacked := filepath.Join(staging, "unpacked-dxvk")
	if err := extract(ctx, archive, unpacked); err != nil {
		return err
	}
	for _, bits := range []DXVKBits{DXVK64, DXVK32} {
		source, err := findDir(unpacked, string(bits))
		if err != nil {
			return err
		}
		target := filepath.Join(staging, "dxvk", string(bits))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Rename(source, target); err != nil {
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
		return "", failure.Errorf(failure.CodeExtractFailed, "the archive has no %s folder", name)
	}
	return found, nil
}
