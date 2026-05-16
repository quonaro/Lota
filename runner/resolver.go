package runner

import (
	"lota/config"
	"runtime"
)

// VarsToEnv converts a map of variables to environment variable format
func VarsToEnv(vars map[string]string) []string {
	rs := make([]string, 0, len(vars))
	for name, value := range vars {
		rs = append(rs, name+"="+value)
	}
	return rs
}

// MergeVarsAndArgs merges vars and args into a single map.
// CLI args override config vars on key collision.
func MergeVarsAndArgs(vars, args map[string]string) map[string]string {
	result := make(map[string]string, len(vars)+len(args))
	for k, v := range vars {
		result[k] = v
	}
	for k, v := range args {
		result[k] = v
	}
	return result
}

// ResolveVars merges variables from all scopes for a specific command.
// Priority: app vars < group1 vars < group2 vars < ... < command vars
func ResolveVars(app config.AppConfig, groups []*config.Group, command config.Command) map[string]string {
	result := make(map[string]string)

	// 1. App level variables (lowest priority)
	for _, v := range app.Vars {
		result[v.Name] = v.Value
	}

	// 2. Group level variables (outermost to innermost)
	for _, g := range groups {
		for _, v := range g.Vars {
			result[v.Name] = v.Value
		}
	}

	// 3. Command level variables (highest priority)
	for _, v := range command.Vars {
		result[v.Name] = v.Value
	}

	return result
}

// mergeArg appends or replaces an arg in result based on priority.
func mergeArg(result []config.Arg, seen map[string]bool, arg config.Arg) []config.Arg {
	if !seen[arg.Name] {
		seen[arg.Name] = true
		return append(result, arg)
	}
	for i, existing := range result {
		if existing.Name == arg.Name {
			result[i] = arg
			break
		}
	}
	return result
}

// ResolveArgs merges arguments from all scopes for a specific command.
// Priority: app args < group1 args < group2 args < ... < command args
func ResolveArgs(app config.AppConfig, groups []*config.Group, command config.Command) []config.Arg {
	result := make([]config.Arg, 0)
	seen := make(map[string]bool)

	// 1. App level arguments (lowest priority)
	for _, arg := range app.Args {
		result = mergeArg(result, seen, arg)
	}

	// 2. Group level arguments (outermost to innermost)
	for _, g := range groups {
		for _, arg := range g.Args {
			result = mergeArg(result, seen, arg)
		}
	}

	// 3. Command level arguments (highest priority)
	for _, arg := range command.Args {
		result = mergeArg(result, seen, arg)
	}

	return result
}

func ResolveDir(app config.AppConfig, groups []*config.Group, command config.Command) string {
	// 1. Group level dir (outermost to innermost)
	var dir string
	for _, g := range groups {
		if g.Dir != "" {
			dir = g.Dir
		}
	}

	// 2. Command level dir (highest priority)
	if command.Dir != "" {
		dir = command.Dir
	}

	return dir
}

func ResolveShell(app config.AppConfig, groups []*config.Group, command config.Command) string {
	// 1. App level shell (lowest priority)
	shell := app.Shell

	// 2. Group level shell (outermost to innermost)
	for _, g := range groups {
		if g.Shell != "" {
			shell = g.Shell
		}
	}

	// 3. Command level shell (highest priority)
	if command.Shell != "" {
		shell = command.Shell
	}

	if shell == "" {
		shell = standardShell(nil)
	}

	return normalizeShell(shell)
}

func normalizeShell(shell string) string {
	var r string

	switch shell {
	case "bash", "sh", "zsh", "dash", "ksh", "mksh", "pdksh", "ash", "busybox", "sash", "tcsh", "csh", "fish":
		r = "-c"
	case "powershell.exe", "pwsh", "powershell":
		r = "-NoProfile -ExecutionPolicy Bypass -Command"
	case "cmd", "cmd.exe":
		r = "/c"
	default:
		return shell
	}
	return shell + " " + r
}

func standardShell(os *string) string {
	if os == nil {
		goos := runtime.GOOS
		os = &goos
	}

	ossh := map[string]string{
		"windows": "powershell.exe",
		"linux":   "bash",
		"darwin":  "bash",
	}

	shell := normalizeShell(ossh[*os])

	return shell
}
