package cli

import (
	"strings"

	"github.com/ThallesP/macron/tools/sandbox/internal/sandbox"
)

type flagKind int

const (
	boolFlag flagKind = iota
	valueFlag
)

type arguments struct {
	flags       map[string]string
	positionals []string
}

func (a arguments) has(name string) bool {
	_, set := a.flags[name]
	return set
}

func parseArguments(raw []string, known map[string]flagKind) (arguments, error) {
	parsed := arguments{flags: map[string]string{}}
	for i := 0; i < len(raw); i++ {
		arg := raw[i]
		if arg == "--" {
			parsed.positionals = append(parsed.positionals, raw[i+1:]...)
			return parsed, nil
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			parsed.positionals = append(parsed.positionals, arg)
			continue
		}

		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		kind, isKnown := known[name]
		if !isKnown {
			return arguments{}, sandbox.Errorf(sandbox.CodeInvalidInput, "unknown flag %s (put a command's own flags after --)", arg)
		}
		if kind == boolFlag {
			parsed.flags[name] = "true"
			continue
		}
		if !hasValue && i+1 == len(raw) {
			return arguments{}, sandbox.Errorf(sandbox.CodeInvalidInput, "%s needs a value", arg)
		}
		if !hasValue {
			i++
			value = raw[i]
		}
		parsed.flags[name] = value
	}
	return parsed, nil
}
