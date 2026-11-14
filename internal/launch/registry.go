package launch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ThallesP/macron/fixes"
	"github.com/ThallesP/macron/internal/failure"
)

const registryMarker = ".macron-registry.json"

func EnsureRegistry(ctx context.Context, in Input, env []string) error {
	want, err := json.Marshal(in.Profile.Registry)
	if err != nil {
		return err
	}
	markerPath := filepath.Join(in.Prefix, registryMarker)
	if have, err := os.ReadFile(markerPath); err == nil && bytes.Equal(have, want) {
		return nil
	}
	if len(in.Profile.Registry) == 0 {
		return os.WriteFile(markerPath, want, 0o644)
	}

	in.Progress("writing the profile's registry values")
	regFile := filepath.Join(in.Prefix, "drive_c", "macron-profile.reg")
	if err := os.WriteFile(regFile, []byte(RegFile(in.Profile.Registry)), 0o644); err != nil {
		return err
	}
	defer os.Remove(regFile)
	tree := backendTrees[in.Profile.Backend]
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	regedit := exec.CommandContext(ctx, in.Runtime.Wine(tree), "regedit", "/S", `C:\macron-profile.reg`)
	regedit.Env = env
	regedit.Stdout = in.Output
	regedit.Stderr = in.Output
	if err := regedit.Run(); err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "importing the profile's registry values: %s", err)
	}
	waitForWineserver(ctx, in.Runtime.Wineserver(tree), env)
	return os.WriteFile(markerPath, want, 0o644)
}

func RegFile(values []fixes.RegistryValue) string {
	var b strings.Builder
	b.WriteString("Windows Registry Editor Version 5.00\r\n")
	key := ""
	for _, value := range values {
		if value.Key != key {
			key = value.Key
			fmt.Fprintf(&b, "\r\n[%s]\r\n", key)
		}
		if value.Type == "dword" {
			fmt.Fprintf(&b, "\"%s\"=dword:%08s\r\n", value.Name, strings.ToLower(value.Value))
			continue
		}
		escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value.Value)
		fmt.Fprintf(&b, "\"%s\"=\"%s\"\r\n", value.Name, escaped)
	}
	return b.String()
}
