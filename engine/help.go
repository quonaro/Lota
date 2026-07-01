package engine

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"github.com/quonaro/lota/config"
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
