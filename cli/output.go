package cli

import (
	"fmt"
	"lota/config"
	"lota/shared"
	"os"
	"path/filepath"
	"strings"

	"github.com/common-nighthawk/go-figure"
	"github.com/fatih/color"
)

const defaultInitTemplate = `# lota.yml

vars:
- PROJECT=myproject

hello:
  desc: Print a greeting
  script: echo "Hello, {{PROJECT}}!"
`

// PrintError prints a formatted error message to stderr
func PrintError(message string) {
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", message)
}

// PrintErrorf prints a formatted error message to stderr
func PrintErrorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
}

// PrintVersion prints version information
func PrintVersion() {
	figure.NewFigure(shared.AppName, "slant", true).Print()
	fmt.Println()
	color.Cyan("version %s\n", shared.Version)
}

// printGlobalOptions prints the global options section
func printGlobalOptions() {
	fmt.Println("Global Options:")
	fmt.Println("  -h, --help       Print help information")
	fmt.Println("  -v, --verbose    Enable detailed logging")
	fmt.Println("      --dry-run    Show interpolated scripts without executing")
	fmt.Println("  -V, --version    Print version information")
	fmt.Println("      --init       Create a default lota.yml in current directory")
	fmt.Println("      --config     Path to config file or directory")
}

// PrintHelp displays available commands
func PrintHelp(configPath string) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n\n", err)
		printGlobalOptions()
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
	printGlobalOptions()
}

// InitConfig creates a default lota.yml at the given path (or current dir if empty)
func InitConfig(configPath string) error {
	path := configPath
	if path == "" {
		path = shared.ConfigFileName
	} else if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, shared.ConfigFileName)
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	}

	if err := os.WriteFile(path, []byte(defaultInitTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}

// PrintCommandHelp displays help for a specific command
func PrintCommandHelp(result config.SearchResult) {
	if result.Command == nil {
		return
	}

	cmd := *result.Command
	cmdName := cmd.Name
	if len(result.Groups) > 0 {
		parts := make([]string, 0, len(result.Groups)+1)
		for _, g := range result.Groups {
			parts = append(parts, g.Name)
		}
		parts = append(parts, cmdName)
		cmdName = strings.Join(parts, " ")
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

	if cmd.Script != "" {
		fmt.Println("Script:")
		fmt.Println("  " + cmd.Script)
		fmt.Println()
	}

	if cmd.Before != "" {
		fmt.Println("Before hook:")
		fmt.Println("  " + cmd.Before)
		fmt.Println()
	}

	if cmd.After != "" {
		fmt.Println("After hook:")
		fmt.Println("  " + cmd.After)
		fmt.Println()
	}
	printGlobalOptions()
}

// PrintGroupHelp displays help for a specific group
func PrintGroupHelp(group *config.Group) {
	fmt.Println("Commands:")

	for _, sub := range group.Groups {
		fmt.Printf("  %-10s %s\n", sub.Name, sub.Desc)
	}

	for _, cmd := range group.Commands {
		fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Desc)
	}

	fmt.Println()
	printGlobalOptions()
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
