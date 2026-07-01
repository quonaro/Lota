package complete

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/posener/complete/v2"
)

// Run runs the completion algorithm for the given command and arguments.
func Run(cmd complete.Completer, args []Arg) ([]string, error) {
	options, err := completer{Completer: cmd, args: args}.complete()
	if err != nil {
		return nil, err
	}
	return sortCompletionOptions(options), nil
}

func sortCompletionOptions(options []string) []string {
	options = uniqueStrings(options)
	sort.SliceStable(options, func(i, j int) bool {
		ci := completionCategory(options[i])
		cj := completionCategory(options[j])
		if ci != cj {
			return ci < cj
		}
		return options[i] < options[j]
	})
	return options
}

func completionCategory(option string) int {
	if strings.HasPrefix(option, "--") {
		return 1
	}
	if strings.HasPrefix(option, "-") {
		return 2
	}
	return 0
}

func (c completer) shouldAutoDescend(token string) bool {
	if token == "" {
		return false
	}

	if c.SubCmdGet(token) == nil {
		return false
	}

	matches := 0
	for _, sub := range c.SubCmdList() {
		if strings.HasPrefix(sub, token) {
			matches++
			if matches > 1 {
				return false
			}
		}
	}

	return matches == 1
}

type completer struct {
	complete.Completer
	args  []Arg
	stack []complete.Completer
}

func (c completer) complete() ([]string, error) {
reset:
	arg := Arg{}
	if len(c.args) > 0 {
		arg = c.args[0]
	}
	switch {
	case len(c.SubCmdList()) == 0:
		// No sub commands, parse flags and positional arguments.
		return c.suggestLeafCommandOptions(), nil

	case !arg.Completed:
		if c.shouldAutoDescend(arg.Text) {
			c.stack = append([]complete.Completer{c.Completer}, c.stack...)
			c.Completer = c.SubCmdGet(arg.Text)
			c.args = c.args[1:]
			goto reset
		}
		// Currently typing in command context: suggest subcommands first,
		// then available flags from current scope stack.
		return c.suggestCommandContextOptions(arg), nil

	case arg.HasFlag:
		// Completed flag token on a command that also has subcommands.
		// Consume it and continue parsing the next argument instead of
		// treating it as an unknown subcommand.
		c.args = c.args[1:]
		goto reset

	case c.SubCmdGet(arg.Text) != nil:
		// Sub command completed, look into that sub command.
		c.stack = append([]complete.Completer{c.Completer}, c.stack...)
		c.Completer = c.SubCmdGet(arg.Text)
		c.args = c.args[1:]
		goto reset

	default:
		// Sub command is unknown...
		return nil, fmt.Errorf("unknown subcommand: %s", arg.Text)
	}
}

func (c completer) suggestSubCommands(prefix string) []string {
	subs := c.SubCmdList()
	return suggest("", prefix, func(prefix string) []string {
		var options []string
		for _, sub := range subs {
			if strings.HasPrefix(sub, prefix) {
				options = append(options, sub)
			}
		}
		return options
	})
}

func (c completer) usedFormattedFlags() map[string]struct{} {
	used := make(map[string]struct{})
	for _, arg := range c.args {
		if !arg.HasFlag || arg.Flag == "" {
			continue
		}
		used[formatFlag(arg.Flag)] = struct{}{}
	}
	return used
}

func filterUsedFlags(options []string, used map[string]struct{}) []string {
	if len(used) == 0 {
		return options
	}
	filtered := make([]string, 0, len(options))
	for _, option := range options {
		if _, exists := used[option]; exists {
			continue
		}
		filtered = append(filtered, option)
	}
	return filtered
}

func (c completer) suggestCommandContextOptions(arg Arg) []string {
	if arg.HasFlag {
		return c.suggestFlag(arg.Dashes, arg.Flag)
	}

	options := c.suggestSubCommands(arg.Text)
	options = append(options, c.suggestFlag("", arg.Text)...)
	return uniqueStrings(options)
}

func (c completer) suggestLeafCommandOptions() (options []string) {
	arg, before := Arg{}, Arg{}
	if len(c.args) > 0 {
		arg = c.args[len(c.args)-1]
	}
	if len(c.args) > 1 {
		before = c.args[len(c.args)-2]
	}

	if !arg.Completed {
		// Complete value being typed.
		if arg.HasValue {
			// Complete value of current flag.
			if arg.HasFlag {
				return c.suggestFlagValue(arg.Flag, arg.Value)
			}
			// Complete value of flag in a previous argument.
			if before.HasFlag && !before.HasValue {
				return c.suggestFlagValue(before.Flag, arg.Value)
			}
		}

		// Suggest positional argument first, then flags.
		if !arg.HasFlag {
			options = append(options, c.suggestArgsValue(arg.Value)...)
		}
		if !arg.HasValue {
			options = append(options, c.suggestFlag(arg.Dashes, arg.Flag)...)
		}
		return options
	}

	// Has a value that was already completed. Suggest positional arguments first, then flags.
	if arg.HasValue {
		if !arg.HasFlag {
			options = append(options, c.suggestArgsValue("")...)
		}
		options = append(options, c.suggestFlag(arg.Dashes, "")...)
		return options
	}
	// A flag without a value. Suggest a value or suggest any flag.
	options = c.suggestFlagValue(arg.Flag, "")
	if len(options) > 0 {
		return options
	}
	return c.suggestFlag("", "")
}

func (c completer) suggestFlag(dashes, prefix string) []string {
	if dashes == "" {
		dashes = "-"
	}
	used := c.usedFormattedFlags()
	options := suggest(dashes, prefix, func(prefix string) []string {
		var options []string
		for _, cmd := range c.flagCompleters() {
			for _, name := range cmd.FlagList() {
				if !flagMatchesDashStyle(name, dashes) {
					continue
				}
				if strings.HasPrefix(name, prefix) {
					options = append(options, formatFlag(name))
				}
			}
		}
		return options
	})
	return filterUsedFlags(options, used)
}

func (c completer) flagCompleters() []complete.Completer {
	completers := append([]complete.Completer{c.Completer}, c.stack...)
	if len(c.stack) > 0 {
		// First item in stack traversal chain is root command, which contains global flags.
		// Hide global flags once we are inside any subcommand/group context.
		return completers[:len(completers)-1]
	}
	return completers
}

func flagMatchesDashStyle(name, dashes string) bool {
	if dashes == "--" {
		return len(name) > 1
	}
	return true
}

func formatFlag(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

func (c completer) suggestFlagValue(flagName, prefix string) []string {
	var options []string
	for _, cmd := range c.flagCompleters() {
		if len(options) == 0 {
			if p := cmd.FlagGet(flagName); p != nil {
				options = p.Predict(prefix)
			}
		}
	}
	return filterByPrefix(prefix, options...)
}

func (c completer) suggestArgsValue(prefix string) []string {
	var options []string
	c.iterateStack(func(cmd complete.Completer) {
		if len(options) == 0 {
			if p := cmd.ArgsGet(); p != nil {
				options = p.Predict(prefix)
			}
		}
	})
	return filterByPrefix(prefix, options...)
}

func (c completer) iterateStack(f func(complete.Completer)) {
	for _, cmd := range append([]complete.Completer{c.Completer}, c.stack...) {
		f(cmd)
	}
}

func suggest(_ string, prefix string, collect func(prefix string) []string) []string {
	options := collect(prefix)
	if len(options) > 0 {
		return uniqueStrings(options)
	}

	// Nothing matched.
	options = collect("")
	return uniqueStrings(options)
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func filterByPrefix(prefix string, options ...string) []string {
	var filtered []string
	for _, option := range options {
		if fixed, ok := hasPrefix(option, prefix); ok {
			filtered = append(filtered, fixed)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return options
}

func hasPrefix(s, prefix string) (string, bool) {
	var (
		token  Tokener
		si, pi int
	)
	for ; pi < len(prefix); pi++ {
		token.Visit(prefix[pi])
		lastQuote := !token.Escaped() && (prefix[pi] == '"' || prefix[pi] == '\'')
		if lastQuote {
			continue
		}
		if si == len(s) {
			break
		}
		if s[si] == ' ' && !token.Quoted() && token.Escaped() {
			s = s[:si] + "\\" + s[si:]
		}
		if s[si] != prefix[pi] {
			return "", false
		}
		si++
	}

	if pi < len(prefix) {
		return "", false
	}

	for ; si < len(s); si++ {
		token.Visit(s[si])
	}

	return token.Closed(), true
}

// ExtractCommandArgs strips the binary name and returns only the command-line arguments.
func ExtractCommandArgs(parsedArgs []Arg, commandName string) []Arg {
	if len(parsedArgs) == 0 {
		return nil
	}

	idx := -1
	for i := len(parsedArgs) - 1; i >= 0; i-- {
		token := parsedArgs[i].Text
		if token == commandName || filepath.Base(token) == commandName {
			idx = i
			break
		}
	}

	if idx >= 0 {
		return parsedArgs[idx+1:]
	}

	// Fallback to previous behavior: assume first token is command.
	return parsedArgs[1:]
}
