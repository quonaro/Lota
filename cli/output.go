package cli

import (
	"errors"
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
  script: echo "Hello, $PROJECT!"
`

var ansiColors = map[string]func(string, ...interface{}) string{
	"black":     color.BlackString,
	"red":       color.RedString,
	"green":     color.GreenString,
	"yellow":    color.YellowString,
	"blue":      color.BlueString,
	"magenta":   color.MagentaString,
	"cyan":      color.CyanString,
	"white":     color.WhiteString,
	"hiblack":   color.HiBlackString,
	"hired":     color.HiRedString,
	"higreen":   color.HiGreenString,
	"hiyellow":  color.HiYellowString,
	"hiblue":    color.HiBlueString,
	"himagenta": color.HiMagentaString,
	"hicyan":    color.HiCyanString,
	"hiwhite":   color.HiWhiteString,
}

// resolveColor returns the effective color name for an object,
// considering its own color and inherited color from ancestor groups.
func resolveColor(objColor string, inheritColor *bool, ancestors []*config.Group) string {
	if objColor != "" {
		return objColor
	}
	if inheritColor != nil && *inheritColor {
		for i := len(ancestors) - 1; i >= 0; i-- {
			if ancestors[i].Color != "" {
				return ancestors[i].Color
			}
		}
	}
	return ""
}

// colorize applies an ANSI color to text if the color name is valid.
func colorize(text, colorName string) string {
	if colorName == "" {
		return text
	}
	fn, ok := ansiColors[strings.ToLower(colorName)]
	if !ok {
		return text
	}
	return fn(text)
}

// paddedName prints a colored name with fixed visual width, preserving alignment.
func paddedName(name, colorName string, width int) string {
	colored := colorize(name, colorName)
	padding := width - len(name)
	if padding < 0 {
		padding = 0
	}
	return colored + strings.Repeat(" ", padding)
}

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

// PrintVersionShort prints a short, machine-friendly version string
func PrintVersionShort() {
	fmt.Println(shared.Version)
}

// printGlobalOptions prints the global options section grouped by category
func printGlobalOptions() {
	fmt.Println("Global Options:")
	fmt.Println()

	fmt.Println("  Information:")
	fmt.Printf("    %-20s %s\n", "-h, --help", "Print help information")
	fmt.Printf("    %-20s %s\n", "-V, --version", "Print version information")
	fmt.Println()

	fmt.Println("  Execution:")
	fmt.Printf("    %-20s %s\n", "-v, --verbose", "Enable detailed logging")
	fmt.Printf("    %-20s %s\n", "--dry-run", "Show interpolated scripts without executing")
	fmt.Printf("    %-20s %s\n", "--timeout", "Set execution timeout (e.g. 30s, 1m)")
	fmt.Println()

	fmt.Println("  Configuration:")
	fmt.Printf("    %-20s %s\n", "--config", "Path to config file or directory")
	fmt.Printf("    %-20s %s\n", "--init", "Create a default lota.yml in current directory")
	fmt.Println()

	fmt.Println("  Maintenance:")
	fmt.Printf("    %-20s %s\n", "-U, --update", "Update lota to the latest version")
	fmt.Printf("    %-20s %s\n", "--completion-script [SHELL]", "Print shell completion script")
	fmt.Printf("    %-20s %s\n", "--install-completion [SHELL]", "Install shell completion (auto-detects if no shell given)")
}

// PrintHelp displays available commands
func PrintHelp(configPath string) {
	cfg, err := LoadConfig(configPath)
	if err != nil {
		if errors.Is(err, config.ErrConfigNotFound) {
			fmt.Println("No lota.yml found in current directory or parent directories.")
			fmt.Println("Run `lota --init` to create a default configuration.")
			fmt.Println()
			printGlobalOptions()
		} else {
			// Show the actual error (parsing, validation, etc.)
			color.Red("Error: %v\n", err)
			fmt.Println()
			printGlobalOptions()
		}
		return
	}

	fmt.Println("Commands:")

	for _, group := range cfg.Groups {
		c := resolveColor(group.Color, group.InheritColor, nil)
		fmt.Printf("  %s %s\n", paddedName(group.Name, c, 18), group.Desc)
	}

	for _, cmd := range cfg.Commands {
		c := resolveColor(cmd.Color, cmd.InheritColor, nil)
		fmt.Printf("  %s %s\n", paddedName(cmd.Name, c, 18), cmd.Desc)
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
	if cmd.Dir != "" {
		fmt.Println("Directory:")
		fmt.Println("  " + cmd.Dir)
		fmt.Println()
	}

	if len(cmd.Depends) > 0 {
		fmt.Println("Depends:")
		for _, dep := range cmd.Depends {
			fmt.Printf("  - %s\n", dep)
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
}

func usageArgName(name string) string {
	if name == "" {
		return "ARG"
	}
	upper := strings.ToUpper(name)
	b := strings.Builder{}
	b.Grow(len(upper))
	for i := 0; i < len(upper); i++ {
		ch := upper[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			b.WriteByte(ch)
			continue
		}
		b.WriteByte('_')
	}
	value := strings.Trim(b.String(), "_")
	if value == "" {
		return "ARG"
	}
	return value
}

func formatPositionalUsage(arg config.Arg) string {
	name := usageArgName(arg.Name)

	if arg.Wildcard {
		return "[...<" + name + ">]"
	}

	value := "<" + name + ">"
	if arg.Type == "arr" {
		value = "<" + name + "...>"
	}

	if arg.Required {
		return value
	}

	return "[" + value + "]"
}

func buildCommandUsage(cmdName string, positionalArgs, flagArgs []config.Arg) string {
	parts := []string{fmt.Sprintf("Usage: %s %s", strings.ToLower(shared.AppName), cmdName)}
	if len(flagArgs) > 0 {
		parts = append(parts, "[OPTIONS]")
	}
	for _, arg := range positionalArgs {
		parts = append(parts, formatPositionalUsage(arg))
	}
	return strings.Join(parts, " ")
}

// PrintCommandHelp displays help for a specific command
func PrintCommandHelp(cfg *config.AppConfig, result config.SearchResult, verbose bool) {
	if result.Command == nil {
		return
	}

	cmd := *result.Command
	cmdName := buildCommandName(cmd, result.Groups)
	c := resolveColor(cmd.Color, cmd.InheritColor, result.Groups)
	coloredCmdName := colorize(cmdName, c)

	// Resolve all arguments and determine their origin scope
	args := runner.ResolveArgs(*cfg, result.Groups, cmd)

	// Separate positional arguments from flags/options
	positionalArgs, flagArgs := separateArgs(args)

	fmt.Println(buildCommandUsage(coloredCmdName, positionalArgs, flagArgs))

	fmt.Println()
	if cmd.Desc != "" {
		fmt.Println(cmd.Desc)
		fmt.Println()
	}

	printCommandArgs(positionalArgs, flagArgs, *cfg, result.Groups, cmd, verbose)

	if verbose {
		printCommandScripts(cmd)
		printGlobalOptions()
	}
}

// PrintGroupHelp displays help for a specific group
func PrintGroupHelp(group *config.Group, ancestors []*config.Group, verbose bool) {
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
		subAncestors := append([]*config.Group(nil), ancestors...)
		subAncestors = append(subAncestors, group)
		c := resolveColor(sub.Color, sub.InheritColor, subAncestors)
		fmt.Printf("  %s %s\n", paddedName(sub.Name, c, 18), sub.Desc)
	}

	for _, cmd := range group.Commands {
		cmdAncestors := append([]*config.Group(nil), ancestors...)
		cmdAncestors = append(cmdAncestors, group)
		c := resolveColor(cmd.Color, cmd.InheritColor, cmdAncestors)
		fmt.Printf("  %s %s\n", paddedName(cmd.Name, c, 18), cmd.Desc)
	}

	if verbose {
		fmt.Println()
		printGlobalOptions()
	}
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
	// Wildcard arguments are always positional
	if arg.Wildcard {
		return false
	}
	// Has short form (e.g., -p)
	if arg.Short != "" {
		return true
	}
	// Boolean flag
	if arg.Type == "bool" {
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
