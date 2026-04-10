package cli

import (
	"fmt"
	"lota/config"
	"lota/shared"
	"os"
	"strings"
)

// PrintError prints a formatted error message and exits
func PrintError(message string) {
	fmt.Printf("ERROR: %s\n", message)
	os.Exit(1)
}

// PrintErrorf prints a formatted error message with printf-style formatting
func PrintErrorf(format string, args ...interface{}) {
	fmt.Printf("ERROR: "+format+"\n", args...)
	os.Exit(1)
}

// PrintVersion prints version information
func PrintVersion() {
	fmt.Printf("%s version %s\n", shared.AppName, shared.AppVersion)
	fmt.Println(shared.AppDescription)
}

// PrintHelp displays available commands
func PrintHelp() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	fmt.Println("Commands:")

	for _, group := range cfg.Groups {
		fmt.Printf("  %-10s %s\n", group.Name, group.Desc)
	}

	for _, cmd := range cfg.Commands {
		fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Desc)
	}

	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  -h, --help     Print help information")
	fmt.Println("  -v, --verbose  Enable detailed logging for debugging")
}

// PrintCommandHelp displays help for a specific command
func PrintCommandHelp(result config.SearchResult) {
	if result.Command == nil {
		return
	}

	cmd := *result.Command
	cmdName := cmd.Name
	if result.Group != nil {
		cmdName = result.Group.Name + " " + cmdName
	}

	fmt.Printf("Usage: %s %s [ARGS]\n", strings.ToLower(shared.AppName), cmdName)
	fmt.Println()
	if cmd.Desc != "" {
		fmt.Println(cmd.Desc)
		fmt.Println()
	}

	if len(cmd.Args) > 0 {
		fmt.Println("Arguments:")
		for _, arg := range cmd.Args {
			argStr := arg.Name
			if arg.Short != "" {
				argStr += "|" + arg.Short
			}
			if arg.Type != "" {
				argStr += ":" + arg.Type
			}
			if arg.Default != "" {
				argStr += "=" + arg.Default
			}
			fmt.Printf("  %-20s %s\n", argStr, describeArg(arg))
		}
		fmt.Println()
	}

}

// PrintGroupHelp displays help for a specific group
func PrintGroupHelp(group *config.Group) {
	fmt.Println("Commands:")

	for _, cmd := range group.Commands {
		fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Desc)
	}

	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  -h, --help     Print help information")
	fmt.Println("  -v, --verbose  Enable detailed logging for debugging")
}

func describeArg(arg config.Arg) string {
	if arg.Wildcard {
		return "Wildcard argument (captures all remaining args)"
	}
	if arg.Type == "bool" {
		return "Boolean flag"
	}
	if arg.Type == "arr" {
		return "Array argument"
	}
	return "Argument"
}
