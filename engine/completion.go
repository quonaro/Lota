package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/predict"

	"github.com/quonaro/lota/config"
	icomp "github.com/quonaro/lota/internal/complete"
	"github.com/quonaro/lota/runner"
)

// BuildCompletion creates a completion tree from the app configuration.
func BuildCompletion(cfg *config.AppConfig) *complete.Command {
	cmd := &complete.Command{
		Sub:   make(map[string]*complete.Command),
		Flags: make(map[string]complete.Predictor),
	}

	// Global flags
	addGlobalFlags(cmd)

	// Top-level groups and commands
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		cmd.Sub[g.Name] = buildGroupCompletion(g)
	}
	for i := range cfg.Commands {
		c := &cfg.Commands[i]
		cmd.Sub[c.Name] = buildCommandCompletion(c)
	}

	return cmd
}

func buildGroupCompletion(g *config.Group) *complete.Command {
	sub := &complete.Command{
		Sub:   make(map[string]*complete.Command),
		Flags: make(map[string]complete.Predictor),
	}

	// Group-level args as flags
	for _, arg := range g.Args {
		if isFlagArgForCompletion(arg) {
			addArgFlag(sub, arg)
		}
	}

	for i := range g.Groups {
		sg := &g.Groups[i]
		sub.Sub[sg.Name] = buildGroupCompletion(sg)
	}
	for i := range g.Commands {
		c := &g.Commands[i]
		sub.Sub[c.Name] = buildCommandCompletion(c)
	}

	return sub
}

func buildCommandCompletion(c *config.Command) *complete.Command {
	cmd := &complete.Command{
		Flags: make(map[string]complete.Predictor),
	}

	// Command-level args as flags
	for _, arg := range c.Args {
		if isFlagArgForCompletion(arg) {
			addArgFlag(cmd, arg)
		}
	}

	// Positional args: allow anything (files, etc.)
	cmd.Args = predictAnything

	return cmd
}

func addGlobalFlags(cmd *complete.Command) {
	cmd.Flags["v"] = predict.Nothing
	cmd.Flags["verbose"] = predict.Nothing
	cmd.Flags["V"] = predict.Nothing
	cmd.Flags["version"] = predict.Nothing
	cmd.Flags["dry-run"] = predict.Nothing
	cmd.Flags["init"] = predict.Nothing
	cmd.Flags["config"] = predict.Files("*")
	cmd.Flags["completion-script"] = predict.Nothing
	cmd.Flags["install-completion"] = predict.Nothing
	cmd.Flags["timeout"] = predict.Nothing
}

func addArgFlag(cmd *complete.Command, arg config.Arg) {
	if arg.Short != "" {
		cmd.Flags[arg.Short] = predict.Nothing
	}
	cmd.Flags[arg.Name] = predict.Nothing
}

func isFlagArgForCompletion(arg config.Arg) bool {
	if arg.Wildcard {
		return false
	}
	return arg.Short != "" || arg.Type == "bool" || arg.Default != ""
}

// anythingPredictor predicts nothing, allowing shell default completion.
type anythingPredictor struct{}

func (anythingPredictor) Predict(prefix string) []string {
	return nil
}

var predictAnything complete.Predictor = anythingPredictor{}

// Complete returns shell completion options for the given line and cursor position.
// line is the full command line (including the binary name), point is the byte offset of the cursor.
// binaryName is the name of the executable (e.g., "lrs").
func (a *App) Complete(line string, point int, binaryName string) ([]string, string, error) {
	if point > len(line) {
		point = len(line)
	}
	comp := BuildCompletion(a.cfg)
	parsedArgs := icomp.ParseArgs(line[:point])
	parsedArgs = icomp.ExtractCommandArgs(parsedArgs, filepath.Base(binaryName))

	options, err := icomp.Run(comp, parsedArgs)
	if err != nil {
		return nil, "", err
	}
	hint := PositionalCompletionHint(a.cfg, parsedArgs)
	return options, hint, nil
}

func PositionalCompletionHint(cfg *config.AppConfig, parsedArgs []icomp.Arg) string {
	if len(parsedArgs) == 0 {
		return ""
	}

	tokens := make([]string, 0, len(parsedArgs))
	for _, arg := range parsedArgs {
		tokens = append(tokens, arg.Text)
	}

	result, _, lastFound := config.ResolveCommand(cfg, tokens)
	if !result.Exists || result.Command == nil {
		return ""
	}

	cmdArgStart := lastFound + 1
	if cmdArgStart < 0 || cmdArgStart > len(parsedArgs) {
		return ""
	}

	argDefs := runner.ResolveArgs(*cfg, result.Groups, *result.Command)
	return nextPositionalHint(parsedArgs[cmdArgStart:], argDefs)
}

func nextPositionalHint(cmdArgs []icomp.Arg, argDefs []config.Arg) string {
	flagDefs := make(map[string]*config.Arg)
	positionals := make([]config.Arg, 0)
	hasWildcard := false

	for i := range argDefs {
		argDef := &argDefs[i]
		if argDef.Wildcard {
			hasWildcard = true
			continue
		}
		if isFlagArgForCompletion(*argDef) {
			if argDef.Name != "" {
				flagDefs["--"+argDef.Name] = argDef
			}
			if argDef.Short != "" {
				flagDefs["-"+argDef.Short] = argDef
			}
			continue
		}
		positionals = append(positionals, *argDef)
	}

	if len(positionals) == 0 {
		return ""
	}

	positionalIndex := 0
	valueOnlyMode := false

	for i := 0; i < len(cmdArgs); i++ {
		token := cmdArgs[i].Text

		if valueOnlyMode {
			if positionalIndex < len(positionals) {
				positionalIndex++
			} else if !hasWildcard {
				return ""
			}
			continue
		}

		if token == "--" {
			valueOnlyMode = true
			continue
		}

		if strings.HasPrefix(token, "-") && len(token) > 1 {
			flagToken := token
			hasInlineValue := false
			if strings.Contains(flagToken, "=") {
				parts := strings.SplitN(flagToken, "=", 2)
				flagToken = parts[0]
				hasInlineValue = true
			}

			flagDef, ok := flagDefs[flagToken]
			if !ok && !hasInlineValue && strings.HasPrefix(flagToken, "--!") {
				negated := "--" + strings.TrimPrefix(flagToken, "--!")
				flagDef, ok = flagDefs[negated]
			}
			if !ok {
				return ""
			}

			if flagDef.Type != "bool" && !hasInlineValue {
				if i+1 >= len(cmdArgs) {
					return ""
				}
				i++
			}
			continue
		}

		if positionalIndex < len(positionals) {
			positionalIndex++
			continue
		}
		if !hasWildcard {
			return ""
		}
	}

	if positionalIndex >= len(positionals) {
		return ""
	}

	return "expected positional arg: " + positionalPlaceholder(positionals[positionalIndex].Name)
}

func positionalPlaceholder(name string) string {
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
		value = "ARG"
	}
	return "<" + value + ">"
}

// GetCompletionScript returns the completion script content for a given shell and binary name.
func GetCompletionScript(shell, binaryName string) (string, error) {
	scriptTemplate, ok := completionScripts[shell]
	if !ok {
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
	safeName := strings.ReplaceAll(binaryName, "-", "_")
	return fmt.Sprintf(scriptTemplate, binaryName, safeName), nil
}

var completionScripts = map[string]string{
	"bash": `_%[2]s_complete() {
    local line="${COMP_LINE}"
    local point="${COMP_POINT}"
    local -a raw
    local -a filtered
    mapfile -t raw < <(%[1]s __complete "$line" "$point")
    filtered=()
    for item in "${raw[@]}"; do
        if [[ "$item" == __hint__:* ]]; then
            continue
        fi
        filtered+=("$item")
    done
    COMPREPLY=("${filtered[@]}")
}
complete -F _%[2]s_complete %[1]s
`,
	"zsh": `#compdef %[1]s
function _%[2]s {
    local line="${LBUFFER}${RBUFFER}"
    local -a raw
    local -a completions
    local -a hints
    raw=(${(f)"$(%[1]s __complete "$line" "${#LBUFFER}")"})
    completions=()
    hints=()

    local item
    for item in "${raw[@]}"; do
        if [[ "$item" == __hint__:* ]]; then
            hints+=("${item#__hint__:}")
            continue
        fi
        completions+=("$item")
    done

    if (( ${#hints[@]} > 0 )); then
        compadd -x "${(j:; :)hints}"
    fi
    if (( ${#completions[@]} > 0 )); then
        compadd -Q -V %[2]s -a completions
    fi
}
compdef _%[2]s %[1]s
`,
	"fish": `function __%[2]s_complete
    for item in (%[1]s __complete (commandline) (commandline -C))
        if string match -q "__hint__:*" -- $item
            continue
        end
        echo $item
    end
end
complete -c %[1]s -f -a "(__%[2]s_complete)"
`,
}
