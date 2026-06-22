package engine

import (
	"fmt"
	"io"
	"strings"

	"github.com/quonaro/lota/config"
)

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
		_, _ = fmt.Fprintf(w, "  %-20s %s\n", group.Name, group.Desc)
	}
	for _, cmd := range cfg.Commands {
		_, _ = fmt.Fprintf(w, "  %-20s %s\n", cmd.Name, cmd.Desc)
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
		_, _ = fmt.Fprintf(w, "  %-20s %s\n", sub.Name, sub.Desc)
	}
	for _, cmd := range group.Commands {
		_, _ = fmt.Fprintf(w, "  %-20s %s\n", cmd.Name, cmd.Desc)
	}
	_, _ = fmt.Fprintln(w)
}
