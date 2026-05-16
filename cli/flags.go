package cli

import (
	"fmt"
	"strings"
	"time"
)

// GlobalFlags represents global CLI flags
type GlobalFlags struct {
	Help             bool
	Verbose          bool
	Version          bool
	DryRun           bool
	Init             bool
	Config           string
	CompletionScript string
	Timeout          time.Duration // 0 means no timeout
}

// ParseGlobalFlags parses global flags from CLI arguments.
// Returns flags and remaining arguments (excluding global flags).
// Only flags that appear before the first non-flag token (command name) are
// treated as global flags. Everything from the first non-flag token onward
// (including any flags mixed in after it) is returned as remaining args.
func ParseGlobalFlags(args []string) (GlobalFlags, []string, error) {
	flags := GlobalFlags{}
	i := 0

	for i < len(args) {
		arg := args[i]

		if !strings.HasPrefix(arg, "-") {
			break
		}

		known := true
		switch arg {
		case "--help", "-h":
			flags.Help = true
		case "--verbose", "-v":
			flags.Verbose = true
		case "--version", "-V":
			flags.Version = true
		case "--dry-run":
			flags.DryRun = true
		case "--init":
			flags.Init = true
		case "--config":
			if i+1 >= len(args) {
				return GlobalFlags{}, nil, fmt.Errorf("flag --config requires a value")
			}
			i++
			flags.Config = args[i]
		case "--completion-script":
			if i+1 >= len(args) {
				return GlobalFlags{}, nil, fmt.Errorf("flag --completion-script requires a value")
			}
			i++
			flags.CompletionScript = args[i]
		case "--timeout":
			if i+1 >= len(args) {
				return GlobalFlags{}, nil, fmt.Errorf("flag --timeout requires a value")
			}
			i++
			d, err := time.ParseDuration(args[i])
			if err != nil {
				return GlobalFlags{}, nil, fmt.Errorf("invalid --timeout value %q: %w", args[i], err)
			}
			flags.Timeout = d
		default:
			known = false
		}

		if !known {
			break
		}
		i++
	}

	if err := validateFlags(flags); err != nil {
		return GlobalFlags{}, nil, err
	}

	return flags, args[i:], nil
}

// hasHelpFlag checks if --help or -h appears anywhere in the args
func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// hasVerboseFlag checks if --verbose or -v appears anywhere in the args
func hasVerboseFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--verbose" || arg == "-v" {
			return true
		}
	}
	return false
}

// validateFlags checks for conflicting flag combinations
func validateFlags(flags GlobalFlags) error {
	if flags.Init && (flags.Help || flags.Version || flags.Verbose || flags.DryRun || flags.CompletionScript != "" || flags.Timeout > 0) {
		return fmt.Errorf("--init cannot be used with --help, --version, --verbose, --dry-run, --completion-script, or --timeout")
	}
	if flags.CompletionScript != "" && (flags.Help || flags.Version || flags.Init || flags.Verbose || flags.DryRun || flags.Timeout > 0) {
		return fmt.Errorf("--completion-script cannot be used with --help, --version, --init, --verbose, --dry-run, or --timeout")
	}
	return nil
}

// HandleGlobalFlags handles global flags.
// Returns (true, nil) if the program should exit normally,
// (true, err) if it should exit with an error.
func HandleGlobalFlags(flags GlobalFlags) (bool, error) {
	if flags.Help {
		PrintHelp(flags.Config)
		return true, nil
	}

	if flags.Version {
		PrintVersion()
		return true, nil
	}

	if flags.Init {
		if err := InitConfig(flags.Config); err != nil {
			return true, err
		}
		return true, nil
	}

	if flags.CompletionScript != "" {
		if err := PrintCompletionScript(flags.CompletionScript); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}
