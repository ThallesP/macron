package check

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ThallesP/macron/internal/failure"
	"github.com/ThallesP/macron/internal/wineruntime"
)

var mesaLLVMPipe = wineruntime.Archive{
	Name:      "mesa3d-26.2.6-release-msvc.7z",
	URL:       "https://github.com/pal1000/mesa-dist-win/releases/download/26.2.6/mesa3d-26.2.6-release-msvc.7z",
	SHA256:    "b2e00615feaba45942343c265cc8c5f576142ca682c1ac6661922d4b72492877",
	SizeBytes: 55964101,
}

var mesaDLLs = []string{"opengl32.dll", "libgallium_wgl.dll", "libglapi.dll"}

func addMesaLLVMPipe(ctx context.Context, cacheDir, workDir, exe string) ([]string, error) {
	archive, err := wineruntime.Download(ctx, cacheDir, mesaLLVMPipe)
	if err != nil {
		return nil, err
	}
	unpacked := filepath.Join(workDir, "mesa")
	if output, err := exec.CommandContext(ctx, "7z", "x", "-y", "-o"+unpacked, archive).CombinedOutput(); err != nil {
		return nil, failure.Errorf(failure.CodeExtractFailed, "unpacking Mesa: %s: %s", err, tail(output))
	}
	machine, err := PEMachine(exe)
	if err != nil {
		return nil, err
	}
	folder := map[uint16]string{0x8664: "x64", 0x14c: "x86"}[machine]
	if folder == "" {
		return nil, fmt.Errorf("%s is not an x86 or x64 Windows program (machine 0x%x)", exe, machine)
	}

	copied := []string{}
	for _, dll := range mesaDLLs {
		content, err := os.ReadFile(filepath.Join(unpacked, folder, dll))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(filepath.Dir(exe), dll), content, 0o644); err != nil {
			return nil, err
		}
		copied = append(copied, dll)
	}
	if len(copied) == 0 {
		return nil, failure.Errorf(failure.CodeExtractFailed, "Mesa's %s folder has none of %v", folder, mesaDLLs)
	}
	return copied, nil
}

func PEMachine(path string) (uint16, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	header := make([]byte, 64)
	if _, err := file.ReadAt(header, 0); err != nil || header[0] != 'M' || header[1] != 'Z' {
		return 0, fmt.Errorf("%s is not a Windows program", path)
	}
	offset := int64(binary.LittleEndian.Uint32(header[0x3c:]))
	signature := make([]byte, 6)
	if _, err := file.ReadAt(signature, offset); err != nil || string(signature[:4]) != "PE\x00\x00" {
		return 0, fmt.Errorf("%s has no PE header", path)
	}
	return binary.LittleEndian.Uint16(signature[4:]), nil
}
