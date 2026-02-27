package main

import (
	"flag"
	"strings"
)

// parseFlagsLoose parses flags even when positional args appear before flags.
// This keeps UX closer to common CLIs where both of these work:
//
//	deeph agent create planner --provider deepseek
//	deeph agent create --provider deepseek planner
func parseFlagsLoose(fs *flag.FlagSet, args []string) ([]string, error) {
	normalized := make([]string, 0, len(args))
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			rest = append(rest, arg)
			continue
		}

		normalized = append(normalized, arg)
		if strings.Contains(arg, "=") {
			continue
		}

		name := strings.TrimLeft(arg, "-")
		if name == "" {
			continue
		}

		f := fs.Lookup(name)
		if f == nil {
			// Let the standard parser report the unknown flag.
			continue
		}
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			normalized = append(normalized, args[i+1])
			i++
		}
	}

	if err := fs.Parse(normalized); err != nil {
		return nil, err
	}
	if parsedRest := fs.Args(); len(parsedRest) > 0 {
		rest = append(rest, parsedRest...)
	}
	return rest, nil
}
