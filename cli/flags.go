package cli

import "strings"

// GlobalFlags represents global CLI flags
type GlobalFlags struct {
	Help    bool
	Verbose bool
	Version bool
}

// ParseGlobalFlags parses global flags from CLI arguments.
// Returns flags and remaining arguments (excluding global flags).
// --help/-h is only treated as a global flag before the first command name;
// if a command name was already seen, --help is kept in remaining so the
// command-level handler can process it.
func ParseGlobalFlags(args []string) (GlobalFlags, []string) {
	flags := GlobalFlags{}
	remaining := make([]string, 0)
	commandSeen := false

	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			commandSeen = true
			remaining = append(remaining, arg)
			continue
		}
		switch arg {
		case "--help", "-h":
			if !commandSeen {
				flags.Help = true
			} else {
				remaining = append(remaining, arg)
			}
		case "--verbose", "-v":
			flags.Verbose = true
		case "--version", "-V":
			flags.Version = true
		default:
			remaining = append(remaining, arg)
		}
	}

	return flags, remaining
}

// HandleGlobalFlags handles global flags.
// Returns true if program should exit after handling.
func HandleGlobalFlags(flags GlobalFlags) bool {
	if flags.Help {
		PrintHelp()
		return true
	}

	if flags.Version {
		PrintVersion()
		return true
	}

	return false
}
