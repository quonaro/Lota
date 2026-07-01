package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/engine"
	icomp "github.com/quonaro/lota/internal/complete"
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

	comp := engine.BuildCompletion(cfg)

	parsedArgs := icomp.ParseArgs(line[:point])
	parsedArgs = icomp.ExtractCommandArgs(parsedArgs, filepath.Base(os.Args[0]))

	options, err := icomp.Run(comp, parsedArgs)
	if err != nil {
		os.Exit(0)
	}
	for _, option := range options {
		fmt.Println(option)
	}
	if hint := engine.PositionalCompletionHint(cfg, parsedArgs); hint != "" {
		fmt.Println(completionHintPrefix + hint)
	}
	os.Exit(0)
}

// GetCompletionScript returns the completion script content for a given shell.
func GetCompletionScript(shell string) (string, error) {
	return engine.GetCompletionScript(shell, filepath.Base(os.Args[0]))
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
func installPath(shell, binaryName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine home directory: %w", err)
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", binaryName), nil
	case "zsh":
		return filepath.Join(home, ".config", "zsh", "completions", "_"+binaryName), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "completions", binaryName+".fish"), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

// InstallCompletionScript writes the completion script to the standard location.
func InstallCompletionScript(shell string) error {
	binaryName := filepath.Base(os.Args[0])

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

	path, err := installPath(shell, binaryName)
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
