package cli

import (
	"fmt"
	"lota/config"
	"lota/runner"
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
	color.Red("ERROR: %s\n", message)
}

// PrintErrorf prints a formatted error message to stderr
func PrintErrorf(format string, args ...interface{}) {
	color.Red("ERROR: "+format+"\n", args...)
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
	fmt.Printf("  %-20s %s\n", "-h, --help", "Print help information")
	fmt.Printf("  %-20s %s\n", "-v, --verbose", "Enable detailed logging")
	fmt.Printf("  %-20s %s\n", "--dry-run", "Show interpolated scripts without executing")
	fmt.Printf("  %-20s %s\n", "-V, --version", "Print version information")
	fmt.Printf("  %-20s %s\n", "--init", "Create a default lota.yml in current directory")
	fmt.Printf("  %-20s %s\n", "--config", "Path to config file or directory")
}

// PrintHelp displays available commands
func PrintHelp(configPath string) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		color.Red("Error loading config: %v\n\n", err)
		printGlobalOptions()
		return
	}

	fmt.Println("Commands:")

	for _, group := range cfg.Groups {
		fmt.Printf("  %-18s %s\n", group.Name, group.Desc)
	}

	for _, cmd := range cfg.Commands {
		fmt.Printf("  %-18s %s\n", cmd.Name, cmd.Desc)
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

// buildCommandName builds the full command name including group hierarchy
func buildCommandName(cmd config.Command, groups []*config.Group) string {
	cmdName := cmd.Name
	if len(groups) > 0 {
		parts := make([]string, 0, len(groups)+1)
		for _, g := range groups {
			parts = append(parts, g.Name)
		}
		parts = append(parts, cmdName)
		cmdName = strings.Join(parts, " ")
	}
	return cmdName
}

// printCommandArgs prints the positional and flag arguments for a command
func printCommandArgs(positionalArgs, flagArgs []config.Arg, cfg config.AppConfig, groups []*config.Group, cmd config.Command, verbose bool) {
	if len(positionalArgs) > 0 {
		fmt.Println("Arguments:")
		for _, arg := range positionalArgs {
			if verbose {
				scope := determineArgScope(arg, cfg, groups, cmd)
				fmt.Printf("  %s [from: %s]\n", arg.Name, scope)
			} else {
				fmt.Printf("  %s\n", arg.Name)
			}
		}
		fmt.Println()
	}

	if len(flagArgs) > 0 {
		fmt.Println("Options:")
		for _, arg := range flagArgs {
			if verbose {
				scope := determineArgScope(arg, cfg, groups, cmd)
				printOptionVerbose(arg, scope)
			} else {
				printOptionInline(arg)
			}
		}
		fmt.Println()
	}
}

// printCommandScripts prints the script and hooks for a command in verbose mode
func printCommandScripts(cmd config.Command) {
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
}

// PrintCommandHelp displays help for a specific command
func PrintCommandHelp(cfg *config.AppConfig, result config.SearchResult, verbose bool) {
	if result.Command == nil {
		return
	}

	cmd := *result.Command
	cmdName := buildCommandName(cmd, result.Groups)

	fmt.Printf("Usage: %s %s [ARGS]\n", strings.ToLower(shared.AppName), cmdName)
	fmt.Println()
	if cmd.Desc != "" {
		fmt.Println(cmd.Desc)
		fmt.Println()
	}

	// Resolve all arguments and determine their origin scope
	args := runner.ResolveArgs(*cfg, result.Groups, cmd)

	// Separate positional arguments from flags/options
	positionalArgs, flagArgs := separateArgs(args)

	printCommandArgs(positionalArgs, flagArgs, *cfg, result.Groups, cmd, verbose)

	if verbose {
		printCommandScripts(cmd)
		printGlobalOptions()
	}
}

// PrintGroupHelp displays help for a specific group
func PrintGroupHelp(group *config.Group, verbose bool) {
	// Show group description if available
	if group.Desc != "" {
		fmt.Println(group.Desc)
		fmt.Println()
	}

	// Show group variables in verbose mode
	if verbose && len(group.Vars) > 0 {
		fmt.Println("Variables:")
		for _, v := range group.Vars {
			fmt.Printf("  %s=%s\n", v.Name, v.Value)
		}
		fmt.Println()
	}

	// Show group arguments (both positional and flags)
	positionalArgs, flagArgs := separateArgs(group.Args)

	if len(positionalArgs) > 0 {
		fmt.Println("Arguments:")
		for _, arg := range positionalArgs {
			fmt.Printf("  %s\n", arg.Name)
		}
		fmt.Println()
	}

	if len(flagArgs) > 0 {
		fmt.Println("Options:")
		for _, arg := range flagArgs {
			printOptionInline(arg)
		}
		fmt.Println()
	}

	fmt.Println("Commands:")

	for _, sub := range group.Groups {
		fmt.Printf("  %-18s %s\n", sub.Name, sub.Desc)
	}

	for _, cmd := range group.Commands {
		fmt.Printf("  %-18s %s\n", cmd.Name, cmd.Desc)
	}

	if verbose {
		fmt.Println()
		printGlobalOptions()
	}
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


// collectGroupArgs collects arguments defined at group level (excluding overridden by inner groups)
func collectGroupArgs(groups []*config.Group) []config.Arg {
	if len(groups) == 0 {
		return nil
	}

	// Collect all group args, with inner groups overriding outer ones
	seen := make(map[string]bool)
	var result []config.Arg

	for i := len(groups) - 1; i >= 0; i-- {
		for _, arg := range groups[i].Args {
			if !seen[arg.Name] {
				seen[arg.Name] = true
				result = append([]config.Arg{arg}, result...)
			}
		}
	}

	return result
}

// determineArgScope determines where an argument was originally defined
func determineArgScope(arg config.Arg, cfg config.AppConfig, groups []*config.Group, cmd config.Command) string {
	// Check command level first (highest priority)
	for _, cmdArg := range cmd.Args {
		if cmdArg.Name == arg.Name {
			return "Command"
		}
	}

	// Check group level (innermost to outermost)
	for i := len(groups) - 1; i >= 0; i-- {
		for _, groupArg := range groups[i].Args {
			if groupArg.Name == arg.Name {
				return "Group"
			}
		}
	}

	// Check global level
	for _, globalArg := range cfg.Args {
		if globalArg.Name == arg.Name {
			return "Global"
		}
	}

	return "Unknown"
}


// separateArgs separates arguments into positional and flag arguments
func separateArgs(args []config.Arg) (positionalArgs, flagArgs []config.Arg) {
	for _, arg := range args {
		if isFlagArg(arg) {
			flagArgs = append(flagArgs, arg)
		} else {
			positionalArgs = append(positionalArgs, arg)
		}
	}
	return positionalArgs, flagArgs
}

// isFlagArg determines if an argument is a flag (non-positional)
func isFlagArg(arg config.Arg) bool {
	// Has short form (e.g., -p)
	if arg.Short != "" {
		return true
	}
	// Boolean flag
	if arg.Type == "bool" {
		return true
	}
	// Has default value (can be used as flag)
	if arg.Default != "" {
		return true
	}
	// Wildcard argument (captures remaining args)
	if arg.Wildcard {
		return true
	}
	return false
}

// printOptionInline formats and prints a single option in flag format
func printOptionInline(arg config.Arg) {
	var flags []string

	// Add short form if exists
	if arg.Short != "" {
		flags = append(flags, "-"+arg.Short)
	}
	// Add long form
	flags = append(flags, "--"+arg.Name)

	flagStr := strings.Join(flags, ", ")

	fmt.Printf("  %s\n", flagStr)
}

// printOptionVerbose formats and prints a single option with source indicator for verbose mode
func printOptionVerbose(arg config.Arg, scope string) {
	var flags []string

	// Add short form if exists
	if arg.Short != "" {
		flags = append(flags, "-"+arg.Short)
	}
	// Add long form
	flags = append(flags, "--"+arg.Name)

	flagStr := strings.Join(flags, ", ")

	fmt.Printf("  %s [from: %s]\n", flagStr, scope)
}


