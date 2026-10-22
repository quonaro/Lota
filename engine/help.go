package engine

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/runner"
)

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

func parseHexColor(s string) (r, g, b uint8, ok bool) {
	if len(s) != 7 || s[0] != '#' {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), true
}

func colorize(text, colorName string) string {
	if colorName == "" {
		return text
	}
	if r, g, b, ok := parseHexColor(colorName); ok {
		return color.RGB(int(r), int(g), int(b)).Sprint(text)
	}
	fn, ok := ansiColors[strings.ToLower(colorName)]
	if !ok {
		return text
	}
	return fn(text)
}

func resolveColor(objColor string, inheritColor *bool, ancestors []*config.Group) string {
	if objColor != "" {
		return objColor
	}
	if inheritColor != nil && !*inheritColor {
		return ""
	}
	allowed := false
	if inheritColor != nil && *inheritColor {
		allowed = true
	} else {
		for i := len(ancestors) - 1; i >= 0; i-- {
			if ancestors[i].InheritColor != nil {
				if *ancestors[i].InheritColor {
					allowed = true
				}
				break
			}
		}
	}
	if allowed {
		for i := len(ancestors) - 1; i >= 0; i-- {
			if ancestors[i].Color != "" {
				return ancestors[i].Color
			}
		}
	}
	return ""
}

func paddedName(name, colorName string, width int) string {
	colored := colorize(name, colorName)
	padding := width - len(name)
	if padding < 0 {
		padding = 0
	}
	return colored + strings.Repeat(" ", padding)
}

// isHidden checks if Show is explicitly set to false
func isHidden(show *bool) bool {
	return show != nil && !*show
}

// PrintHelp writes a list of available top-level commands and groups to w.
// Unlike cli.PrintHelp, it does not print global CLI flags (--init,
// --completion-script, etc.) because those are irrelevant when Lota is
// embedded into another application.
func PrintHelp(cfg *config.AppConfig, w io.Writer, appName string) {
	if appName == "" {
		appName = "app"
	}
	_, _ = fmt.Fprintf(w, "Usage: %s <command> [args...]\n\n", appName)
	_, _ = fmt.Fprintln(w, "Commands:")

	for _, group := range cfg.Groups {
		if isHidden(group.Show) {
			continue
		}
		c := resolveColor(group.Color, group.InheritColor, nil)
		_, _ = fmt.Fprintf(w, "  %s %s\n", paddedName(group.Name, c, 20), group.Desc)
	}
	for _, cmd := range cfg.Commands {
		if isHidden(cmd.Show) {
			continue
		}
		c := resolveColor(cmd.Color, cmd.InheritColor, nil)
		_, _ = fmt.Fprintf(w, "  %s %s\n", paddedName(cmd.Name, c, 20), cmd.Desc)
	}
	_, _ = fmt.Fprintln(w)
}

// PrintGroupHelp writes the commands and sub-groups inside a specific group to w.
// groups is the chain of groups from outermost to innermost (as returned by
// config.ResolveCommand). If groups is empty, it falls back to PrintHelp.
func PrintGroupHelp(cfg *config.AppConfig, groups []*config.Group, w io.Writer, appName string) {
	if len(groups) == 0 {
		PrintHelp(cfg, w, appName)
		return
	}

	group := groups[len(groups)-1]

	if appName == "" {
		appName = "app"
	}

	pathParts := make([]string, 0, len(groups))
	for _, g := range groups {
		pathParts = append(pathParts, g.Name)
	}
	path := strings.Join(pathParts, " ")

	_, _ = fmt.Fprintf(w, "Usage: %s %s <command> [args...]\n\n", appName, path)

	if group.Desc != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", group.Desc)
	}

	_, _ = fmt.Fprintln(w, "Commands:")

	for _, sub := range group.Groups {
		if isHidden(sub.Show) {
			continue
		}
		subAncestors := append([]*config.Group(nil), groups...)
		c := resolveColor(sub.Color, sub.InheritColor, subAncestors)
		_, _ = fmt.Fprintf(w, "  %s %s\n", paddedName(sub.Name, c, 20), sub.Desc)
	}
	for _, cmd := range group.Commands {
		if isHidden(cmd.Show) {
			continue
		}
		cmdAncestors := append([]*config.Group(nil), groups...)
		c := resolveColor(cmd.Color, cmd.InheritColor, cmdAncestors)
		_, _ = fmt.Fprintf(w, "  %s %s\n", paddedName(cmd.Name, c, 20), cmd.Desc)
	}
	_, _ = fmt.Fprintln(w)
}

// isFlagArg reports whether an argument is a flag (non-positional).
func isFlagArg(arg config.Arg) bool {
	if arg.Wildcard {
		return false
	}
	return arg.Short != "" || arg.Type == "bool"
}

// separateArgs splits arguments into positional and flag arguments.
func separateArgs(args []config.Arg) (positional, flags []config.Arg) {
	for _, arg := range args {
		if isFlagArg(arg) {
			flags = append(flags, arg)
		} else {
			positional = append(positional, arg)
		}
	}
	return positional, flags
}

// usageArgName converts an argument name into an uppercase placeholder.
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
	if value := strings.Trim(b.String(), "_"); value != "" {
		return value
	}
	return "ARG"
}

// formatPositionalUsage renders a single positional argument for the usage line.
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

// PrintCommandHelp writes help for a specific command to w. It mirrors the
// standalone CLI's command help but omits global CLI flags, which are
// irrelevant for embedded applications.
func PrintCommandHelp(cfg *config.AppConfig, result config.SearchResult, w io.Writer, appName string, verbose bool) {
	if result.Command == nil {
		return
	}
	if appName == "" {
		appName = "app"
	}

	cmd := *result.Command

	pathParts := make([]string, 0, len(result.Groups)+1)
	for _, g := range result.Groups {
		pathParts = append(pathParts, g.Name)
	}
	pathParts = append(pathParts, cmd.Name)
	cmdName := strings.Join(pathParts, " ")

	args := runner.ResolveArgs(*cfg, result.Groups, cmd)
	positional, flags := separateArgs(args)

	usage := strings.Builder{}
	usage.WriteString("Usage: ")
	usage.WriteString(appName)
	usage.WriteString(" ")
	usage.WriteString(cmdName)
	if len(flags) > 0 {
		usage.WriteString(" [OPTIONS]")
	}
	for _, arg := range positional {
		usage.WriteString(" ")
		usage.WriteString(formatPositionalUsage(arg))
	}
	_, _ = fmt.Fprintln(w, colorize(usage.String(), resolveColor(cmd.Color, cmd.InheritColor, result.Groups)))
	_, _ = fmt.Fprintln(w)

	if cmd.Desc != "" {
		_, _ = fmt.Fprintf(w, "%s\n\n", cmd.Desc)
	}

	if len(positional) > 0 {
		_, _ = fmt.Fprintln(w, "Arguments:")
		for _, arg := range positional {
			_, _ = fmt.Fprintf(w, "  %s\n", arg.Name)
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(flags) > 0 {
		_, _ = fmt.Fprintln(w, "Options:")
		for _, arg := range flags {
			names := make([]string, 0, 2)
			if arg.Short != "" {
				names = append(names, "-"+arg.Short)
			}
			names = append(names, "--"+arg.Name)
			_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(names, ", "))
		}
		_, _ = fmt.Fprintln(w)
	}

	if verbose {
		if cmd.Dir != "" {
			_, _ = fmt.Fprintf(w, "Directory:\n  %s\n\n", cmd.Dir)
		}
		if len(cmd.Depends) > 0 {
			_, _ = fmt.Fprintln(w, "Depends:")
			for _, dep := range cmd.Depends {
				if dep.Wait != "" {
					_, _ = fmt.Fprintf(w, "  - %s (wait: %s)\n", dep.Name, dep.Wait)
				} else {
					_, _ = fmt.Fprintf(w, "  - %s\n", dep.Name)
				}
			}
			_, _ = fmt.Fprintln(w)
		}
		if cmd.Script != "" {
			_, _ = fmt.Fprintf(w, "Script:\n  %s\n\n", cmd.Script)
		}
		if cmd.Before != "" {
			_, _ = fmt.Fprintf(w, "Before hook:\n  %s\n\n", cmd.Before)
		}
		if cmd.After != "" {
			_, _ = fmt.Fprintf(w, "After hook:\n  %s\n\n", cmd.After)
		}
	}
}
