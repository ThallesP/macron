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

type registryImport struct {
	Values   []fixes.RegistryValue
	Marker   string
	FileName string
	Message  string
}

func EnsureRegistry(ctx context.Context, in Input, env []string) error {
	return importRegistry(ctx, in, env, registryImport{
		Values:   in.Profile.Registry,
		Marker:   registryMarker,
		FileName: "macron-profile.reg",
		Message:  "writing the profile's registry values",
	})
}

func importRegistry(ctx context.Context, in Input, env []string, request registryImport) error {
	want, err := json.Marshal(request.Values)
	if err != nil {
		return err
	}
	markerPath := filepath.Join(in.Prefix, request.Marker)
	if have, err := os.ReadFile(markerPath); err == nil && bytes.Equal(have, want) {
		return nil
	}
	if len(request.Values) == 0 {
		return os.WriteFile(markerPath, want, 0o644)
	}

	in.Progress(request.Message)
	regFile := filepath.Join(in.Prefix, "drive_c", request.FileName)
	if err := os.WriteFile(regFile, []byte(RegFile(request.Values)), 0o644); err != nil {
		return err
	}
	defer os.Remove(regFile)
	tree := backendTrees[in.Profile.Backend]
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	startServerWithoutLinger(ctx, in.Runtime.Wineserver(tree), env)
	regedit := exec.CommandContext(ctx, in.Runtime.Wine(tree), "regedit", "/S", `C:\`+request.FileName)
	regedit.Env = env
	regedit.Stdout = in.Output
	regedit.Stderr = in.Output
	if err := regedit.Run(); err != nil {
		return failure.Errorf(failure.CodePrefixFailed, "%s: %s", request.Message, err)
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
