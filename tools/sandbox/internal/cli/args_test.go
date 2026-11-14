package cli

import (
	"reflect"
	"testing"
)

func TestParseArguments(t *testing.T) {
	known := map[string]flagKind{"json": boolFlag, "ttl": valueFlag, "r": boolFlag}

	cases := []struct {
		name        string
		raw         []string
		flags       map[string]string
		positionals []string
	}{
		{"flags may come after the id", []string{"sbx_a", "--json", "--", "ls", "-la"}, map[string]string{"json": "true"}, []string{"sbx_a", "ls", "-la"}},
		{"everything after -- is the command", []string{"sbx_a", "--", "--json", "--ttl"}, map[string]string{}, []string{"sbx_a", "--json", "--ttl"}},
		{"a value flag takes the next argument", []string{"--ttl", "5m"}, map[string]string{"ttl": "5m"}, nil},
		{"a value flag takes = too", []string{"--ttl=90s", "-r"}, map[string]string{"ttl": "90s", "r": "true"}, nil},
		{"a lone dash is a path", []string{"-", "sbx_a:/tmp/x"}, map[string]string{}, []string{"-", "sbx_a:/tmp/x"}},
	}
	for _, c := range cases {
		got, err := parseArguments(c.raw, known)
		if err != nil {
			t.Errorf("%s: %s", c.name, err)
			continue
		}
		if !reflect.DeepEqual(got.flags, c.flags) || !reflect.DeepEqual(got.positionals, c.positionals) {
			t.Errorf("%s: got flags %v and positionals %v", c.name, got.flags, got.positionals)
		}
	}

	for _, raw := range [][]string{{"sbx_a", "ls", "-la"}, {"--ttl"}, {"--nope"}} {
		if _, err := parseArguments(raw, known); err == nil {
			t.Errorf("%v should be rejected", raw)
		}
	}
}
