package cli

import (
	"fmt"
	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/runner"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/predict"

	icomp "github.com/quonaro/lota/cli/internal/complete"
)

const completionHintPrefix = "__hint__:"

// RunCompleteSubcommand runs shell completion from explicit positional arguments.
// args[0] is the full command line; args[1] is the cursor position.
func RunCompleteSubcommand(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: lota __complete <line> <point>")
		os.Exit(1)
	}

	line := args[0]
	point, err := strconv.Atoi(args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid point: %v\n", err)
		os.Exit(1)
	}
	if point > len(line) {
		point = len(line)
	}

	cfg, err := LoadConfigWithWriter("", io.Discard)
	if err != nil {
		cfg = &config.AppConfig{}
	}

	comp := BuildCompletion(cfg)

	parsedArgs := icomp.ParseArgs(line[:point])
	parsedArgs = extractCompletionArgs(parsedArgs, filepath.Base(os.Args[0]))

	options, err := icomp.Run(comp, parsedArgs)
	if err != nil {
		os.Exit(0)
	}
	for _, option := range options {
		fmt.Println(option)
	}
	if hint := positionalCompletionHint(cfg, parsedArgs); hint != "" {
		fmt.Println(completionHintPrefix + hint)
	}
	os.Exit(0)
}

func extractCompletionArgs(parsedArgs []icomp.Arg, commandName string) []icomp.Arg {
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

func positionalCompletionHint(cfg *config.AppConfig, parsedArgs []icomp.Arg) string {
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

// completionScripts maps shell names to their completion scripts.
var completionScripts = map[string]string{
	"bash": `_lota_complete() {
    local line="${COMP_LINE}"
    local point="${COMP_POINT}"
    local -a raw
    local -a filtered
    mapfile -t raw < <(lota __complete "$line" "$point")
    filtered=()
    for item in "${raw[@]}"; do
        if [[ "$item" == __hint__:* ]]; then
            continue
        fi
        filtered+=("$item")
    done
    COMPREPLY=("${filtered[@]}")
}
complete -F _lota_complete lota
`,
	"zsh": `#compdef lota
function _lota {
    local line="${LBUFFER}${RBUFFER}"
    local -a raw
    local -a completions
    local -a hints
    raw=(${(f)"$(lota __complete "$line" "${#LBUFFER}")"})
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
        compadd -Q -V lota -a completions
    fi
}
compdef _lota lota
`,
	"fish": `function __lota_complete
    for item in (lota __complete (commandline) (commandline -C))
        if string match -q "__hint__:*" -- $item
            continue
        end
        echo $item
    end
end
complete -c lota -f -a "(__lota_complete)"
`,
}

// GetCompletionScript returns the completion script content for a given shell.
func GetCompletionScript(shell string) (string, error) {
	script, ok := completionScripts[shell]
	if !ok {
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
	return script, nil
}

// PrintCompletionScript prints a shell completion installation script.
func PrintCompletionScript(shell string) error {
	script, err := GetCompletionScript(shell)
	if err != nil {
		return err
	}
	fmt.Print(script)
	return nil
}

// detectShell returns the shell name from the $SHELL environment variable.
func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ""
	}
	return filepath.Base(shell)
}

// installPath returns the standard installation path for a completion script.
func installPath(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", "lota"), nil
	case "zsh":
		return filepath.Join(home, ".config", "zsh", "completions", "_lota"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "completions", "lota.fish"), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

// InstallCompletionScript writes the completion script to the standard location.
func InstallCompletionScript(shell string) error {
	if shell == "" {
		shell = detectShell()
		if shell == "" {
			return fmt.Errorf("unable to detect shell; specify explicitly: --install-completion bash|zsh|fish")
		}
	}

	script, err := GetCompletionScript(shell)
	if err != nil {
		return err
	}

	path, err := installPath(shell)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	fmt.Printf("Installed %s completion to %s\n", shell, path)

	if shell == "zsh" {
		if err := ensureZshFpath(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
	}

	return nil
}

// ensureZshFpath checks if ~/.config/zsh/completions is in fpath in ~/.zshrc,
// and appends it before compinit if missing.
func ensureZshFpath() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("unable to determine home directory: %w", err)
	}

	zshrc := filepath.Join(home, ".zshrc")
	data, err := os.ReadFile(zshrc)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", zshrc, err)
	}

	content := string(data)
	marker := "# Lota zsh completion path"
	if strings.Contains(content, marker) {
		return nil // already configured
	}

	compDir := filepath.Join(home, ".config", "zsh", "completions")
	fpathLine := fmt.Sprintf("%s\nfpath+=(%s)", marker, compDir)

	// Try to insert before compinit
	if idx := strings.Index(content, "compinit"); idx != -1 {
		// Find the start of that line
		lineStart := strings.LastIndex(content[:idx], "\n")
		if lineStart == -1 {
			lineStart = 0
		} else {
			lineStart++
		}
		content = content[:lineStart] + fpathLine + "\n" + content[lineStart:]
	} else {
		content = content + "\n" + fpathLine + "\n"
	}

	if err := os.WriteFile(zshrc, []byte(content), 0644); err != nil {
		return fmt.Errorf("could not write %s: %w", zshrc, err)
	}

	fmt.Printf("Updated %s with fpath for zsh completions\n", zshrc)
	fmt.Println("Please reload your shell: exec zsh")
	return nil
}
