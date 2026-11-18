package launch

import (
	"os"
	"path/filepath"
)

// NVIDIA's NGX, which a game with DLSS links, checks the signature of the
// nvngx.dll file in the prefix's System32 before it loads it. Wine runs the
// runtime's nvngx.dll whatever that file holds, but a prefix made by an
// earlier runtime still has that runtime's file, and DXMT trusts only the
// certificate of the nvngx.dll it was built with.
func syncNGXCore(root, prefix string) error {
	source := filepath.Join(root, "lib", "wine", "x86_64-windows", "nvngx.dll")
	if _, err := os.Stat(source); err != nil {
		return nil
	}
	return copyIfChanged(source, filepath.Join(prefix, "drive_c", "windows", "system32", "nvngx.dll"))
}
