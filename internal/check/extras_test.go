package check

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func fakePE(t *testing.T, machine uint16) string {
	t.Helper()
	content := make([]byte, 0x100)
	content[0], content[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(content[0x3c:], 0x80)
	copy(content[0x80:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(content[0x84:], machine)
	path := filepath.Join(t.TempDir(), "game.exe")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPEMachine(t *testing.T) {
	for _, machine := range []uint16{0x8664, 0x14c} {
		got, err := PEMachine(fakePE(t, machine))
		if err != nil || got != machine {
			t.Errorf("got 0x%x, %v; want 0x%x", got, err, machine)
		}
	}
}

func TestPEMachineRejectsOtherFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	os.WriteFile(path, []byte("hello, this is not a program at all, it is a text file of some length"), 0o644)

	if _, err := PEMachine(path); err == nil {
		t.Fatal("a text file was taken for a Windows program")
	}
}
