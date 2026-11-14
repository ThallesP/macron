package wineruntime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ThallesP/macron/internal/failure"
)

var Mesa = Archive{
	Name:      "mesa3d-26.1.8-release-msvc.7z",
	URL:       "https://github.com/pal1000/mesa-dist-win/releases/download/26.1.8/mesa3d-26.1.8-release-msvc.7z",
	SHA256:    "4c6d32e653e0ff9ad07796e40c0bcfabf2764d849e3ce4f3b1590112c87e42f9",
	SizeBytes: 71005136,
}

var MesaDLLs = []string{"opengl32.dll", "libgallium_wgl.dll"}

type MesaBits string

const (
	Mesa64 MesaBits = "x64"
	Mesa32 MesaBits = "x86"
)

func (r Installed) Mesa(bits MesaBits) string {
	return filepath.Join(r.Dir, "mesa", string(bits))
}

func EnsureMesa(ctx context.Context, downloadsDir string, installed Installed, archive Archive, progress func(string)) error {
	marker := filepath.Join(installed.Dir, "mesa", archive.SHA256)
	if _, err := os.Stat(marker); err == nil {
		return nil
	}

	progress("downloading Mesa for software OpenGL")
	path, err := Download(ctx, downloadsDir, archive)
	if err != nil {
		return err
	}
	staging := filepath.Join(installed.Dir, "mesa.partial")
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	members := []string{}
	for _, bits := range []MesaBits{Mesa64, Mesa32} {
		for _, dll := range MesaDLLs {
			members = append(members, string(bits)+"/"+dll)
		}
	}
	output, err := exec.CommandContext(ctx, "tar", append([]string{"-xf", path, "-C", staging}, members...)...).CombinedOutput()
	if err != nil {
		return failure.Errorf(failure.CodeExtractFailed, "unpacking Mesa's OpenGL DLLs: %s: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(staging, archive.SHA256), nil, 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(installed.Dir, "mesa")); err != nil {
		return err
	}
	return os.Rename(staging, filepath.Join(installed.Dir, "mesa"))
}
