package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncNGXCoreReplacesAnEarlierRuntimesFile(t *testing.T) {
	root, prefix := t.TempDir(), t.TempDir()
	source := filepath.Join(root, "lib", "wine", "x86_64-windows", "nvngx.dll")
	target := filepath.Join(prefix, "drive_c", "windows", "system32", "nvngx.dll")
	for path, content := range map[string]string{source: "signed by this runtime", target: "an earlier runtime's"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := syncNGXCore(root, prefix); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "signed by this runtime" {
		t.Errorf("the prefix's nvngx.dll = %q, want the runtime's", got)
	}
}

func TestSyncNGXCoreLeavesAPrefixAloneWhenTheTreeHasNone(t *testing.T) {
	root, prefix := t.TempDir(), t.TempDir()

	if err := syncNGXCore(root, prefix); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(prefix, "drive_c")); err == nil {
		t.Error("a tree without nvngx.dll (the plain Wine tree) still wrote into the prefix")
	}
}
