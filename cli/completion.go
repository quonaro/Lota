package cli

import (
	"fmt"
	"lota/config"
	"os"
	"strconv"

	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/predict"
)

// RunCompletion runs shell completion based on COMP_LINE and COMP_POINT env vars.
func RunCompletion() {
	// posener/complete requires COMP_POINT; default to end of line if missing
	if os.Getenv("COMP_POINT") == "" {
		if line := os.Getenv("COMP_LINE"); line != "" {
			_ = os.Setenv("COMP_POINT", strconv.Itoa(len(line)))
		}
	}

	cfg, err := LoadConfig("")
	if err != nil {
		cfg = &config.AppConfig{}
	}

	comp := BuildCompletion(cfg)
	comp.Complete("lota")
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
		addArgFlag(sub, arg)
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
		addArgFlag(cmd, arg)
	}

	// Positional args: allow anything (files, etc.)
	cmd.Args = predictAnything

	return cmd
}

func addGlobalFlags(cmd *complete.Command) {
	cmd.Flags["-h"] = predict.Nothing
	cmd.Flags["--help"] = predict.Nothing
	cmd.Flags["-v"] = predict.Nothing
	cmd.Flags["--verbose"] = predict.Nothing
	cmd.Flags["-V"] = predict.Nothing
	cmd.Flags["--version"] = predict.Nothing
	cmd.Flags["--dry-run"] = predict.Nothing
	cmd.Flags["--init"] = predict.Nothing
	cmd.Flags["--config"] = predict.Files("*")
}

func addArgFlag(cmd *complete.Command, arg config.Arg) {
	if arg.Short != "" {
		cmd.Flags["-"+arg.Short] = predict.Nothing
	}
	cmd.Flags["--"+arg.Name] = predict.Nothing
}

// anythingPredictor predicts nothing, allowing shell default completion.
type anythingPredictor struct{}

func (anythingPredictor) Predict(prefix string) []string {
	return nil
}

var predictAnything complete.Predictor = anythingPredictor{}

// PrintCompletionScript prints a shell completion installation script.
func PrintCompletionScript(shell string) error {
	switch shell {
	case "bash":
		fmt.Println("complete -C 'lota' lota")
	case "zsh":
		fmt.Println(`#compdef lota
function _lota {
    local line="${LBUFFER}${RBUFFER}"
    export COMP_LINE="$line"
    export COMP_POINT=${#LBUFFER}
    local -a completions
    completions=($('lota'))
    compadd -a completions
}
compdef _lota lota`)
	case "fish":
		fmt.Println(`complete -c lota -f -a "(env COMP_LINE=(commandline) COMP_POINT=(commandline -C) lota)"`)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
	return nil
}
