package cli

import (
	"fmt"
	"strings"
	"time"
)

// GlobalFlags represents global CLI flags
type GlobalFlags struct {
	Help              bool
	Verbose           bool
	VersionShort      bool
	VersionLong       bool
	Update            bool
	DryRun            bool
	Init              bool
	Config            string
	GlobalConfig      bool // -G
	UserConfig        bool // -U
	CompletionScript  string
	InstallCompletion string        // empty means not requested; "auto" means auto-detect shell
	Timeout           time.Duration // 0 means no timeout
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
		case "--version":
			flags.VersionLong = true
		case "-V":
			flags.VersionShort = true
		case "--update":
			flags.Update = true
		case "-u":
			flags.UserConfig = true
		case "--dry-run":
			flags.DryRun = true
		case "--init":
			flags.Init = true
		case "-g":
			flags.GlobalConfig = true
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
		case "--install-completion":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags.InstallCompletion = args[i]
			} else {
				flags.InstallCompletion = "auto"
			}
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
	hasVersion := flags.VersionShort || flags.VersionLong

	if flags.Init && (flags.Help || hasVersion || flags.Update || flags.Verbose || flags.DryRun || flags.CompletionScript != "" || flags.InstallCompletion != "" || flags.Timeout > 0 || flags.GlobalConfig || flags.UserConfig) {
		return fmt.Errorf("--init cannot be used with --help, --version, --update, --verbose, --dry-run, --completion-script, --install-completion, --timeout, -G, or -U")
	}
	if flags.CompletionScript != "" && (flags.Help || hasVersion || flags.Update || flags.Init || flags.Verbose || flags.DryRun || flags.InstallCompletion != "" || flags.Timeout > 0 || flags.GlobalConfig || flags.UserConfig) {
		return fmt.Errorf("--completion-script cannot be used with --help, --version, --update, --init, --verbose, --dry-run, --install-completion, --timeout, -G, or -U")
	}
	if flags.InstallCompletion != "" && (flags.Help || hasVersion || flags.Update || flags.Init || flags.Verbose || flags.DryRun || flags.CompletionScript != "" || flags.Timeout > 0 || flags.GlobalConfig || flags.UserConfig) {
		return fmt.Errorf("--install-completion cannot be used with --help, --version, --update, --init, --verbose, --dry-run, --completion-script, --timeout, -G, or -U")
	}
	if flags.Update && (flags.Help || hasVersion || flags.Init || flags.Verbose || flags.DryRun || flags.CompletionScript != "" || flags.InstallCompletion != "" || flags.Timeout > 0 || flags.Config != "" || flags.GlobalConfig || flags.UserConfig) {
		return fmt.Errorf("--update cannot be used with --help, --version, --init, --verbose, --dry-run, --completion-script, --install-completion, --timeout, --config, -G, or -U")
	}
	if flags.GlobalConfig && flags.Config != "" {
		return fmt.Errorf("-G cannot be used with --config")
	}
	if flags.UserConfig && flags.Config != "" {
		return fmt.Errorf("-U cannot be used with --config")
	}
	if flags.GlobalConfig && flags.UserConfig {
		return fmt.Errorf("-G and -U cannot be used together")
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

	if flags.VersionShort {
		PrintVersionShort()
		return true, nil
	}

	if flags.VersionLong {
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

	if flags.Update {
		if err := PerformUpdate(); err != nil {
			return true, err
		}
		return true, nil
	}

	if flags.InstallCompletion != "" {
		shell := flags.InstallCompletion
		if shell == "auto" {
			shell = ""
		}
		if err := InstallCompletionScript(shell); err != nil {
			return true, err
		}
		return true, nil
	}

	return false, nil
}
