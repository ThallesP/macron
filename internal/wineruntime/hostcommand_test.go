package wineruntime

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestHostCommandDropsDyldVariables(t *testing.T) {
	t.Setenv("DYLD_INSERT_LIBRARIES", "/Steam/steamloader.dylib")
	t.Setenv("DYLD_LIBRARY_PATH", "/Steam")
	t.Setenv("MACRON_KEPT", "yes")

	env := HostCommand(context.Background(), "cp", "a", "b").Env

	if slices.ContainsFunc(env, func(variable string) bool { return strings.HasPrefix(variable, "DYLD_") }) {
		t.Errorf("a host tool would inherit dyld's variables: %v", env)
	}
	if !slices.Contains(env, "MACRON_KEPT=yes") {
		t.Errorf("a host tool lost an ordinary variable")
	}
}
