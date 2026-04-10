package cli

import (
	"fmt"
	"os"
	"strings"
)

// Run executes the CLI application
func Run() error {
	if len(os.Args) < 2 {
		PrintHelp()
		os.Exit(0)
	}

	cliArgs := os.Args[1:]

	flags, remainingArgs := ParseGlobalFlags(cliArgs)

	if HandleGlobalFlags(flags) {
		os.Exit(0)
	}

	if len(remainingArgs) == 0 {
		PrintHelp()
		os.Exit(0)
	}

	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	cmdPath, cmdArgs := ParseCommandPath(remainingArgs)

	result := FindCommand(cfg, cmdPath)
	if !result.Exists {
		PrintErrorf("Command not found: %s", strings.Join(cmdPath, " "))
	}

	if result.Exists && result.Group != nil && result.Command == nil {
		PrintGroupHelp(result.Group)
		os.Exit(0)
	}

	for _, arg := range cmdArgs {
		if arg == "--help" || arg == "-h" {
			PrintCommandHelp(result)
			os.Exit(0)
		}
	}

	return RunCommand(cfg, result, cmdArgs)
}
