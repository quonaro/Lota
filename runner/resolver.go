package runner

import (
	"fmt"
	"lota/config"
)

// VarsToEnv converts a map of variables to environment variable format
func VarsToEnv(vars map[string]string) []string {
	rs := make([]string, 0, len(vars))
	for name, value := range vars {
		rs = append(rs, name+"="+value)
	}
	return rs
}

// ScopedVar represents a variable with its scope information
type ScopedVar struct {
	Scope string
	Path  string
	Var   config.Var
}

// ResolveVars merges variables from all scopes for a specific command.
// Priority: app vars < group vars < command vars (command overrides all)
func ResolveVars(app config.AppConfig, group *config.Group, command config.Command) map[string]string {
	result := make(map[string]string)

	// 1. App level variables (lowest priority)
	for _, v := range app.Vars {
		result[v.Name] = v.Value
	}

	// 2. Group level variables (override app level)
	if group != nil {
		for _, v := range group.Vars {
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
// Priority: app args < group args < command args (command overrides all)
func ResolveArgs(app config.AppConfig, group *config.Group, command config.Command) []config.Arg {
	result := make([]config.Arg, 0)
	seen := make(map[string]bool)

	// 1. App level arguments (lowest priority)
	for _, arg := range app.Args {
		result = mergeArg(result, seen, arg)
	}

	// 2. Group level arguments (override app level)
	if group != nil {
		for _, arg := range group.Args {
			result = mergeArg(result, seen, arg)
		}
	}

	// 3. Command level arguments (highest priority)
	for _, arg := range command.Args {
		result = mergeArg(result, seen, arg)
	}

	return result
}

// CollectAllVars collects all variables from config for debugging/inspection
func CollectAllVars(configs []config.AppConfig) []ScopedVar {
	r := []ScopedVar{}

	for i, app := range configs {
		appPath := fmt.Sprintf("app[%d]", i)
		for _, v := range app.Vars {
			r = append(r, ScopedVar{Scope: "app", Path: appPath, Var: v})
		}

		for _, group := range app.Groups {
			for _, v := range group.Vars {
				r = append(r, ScopedVar{Scope: "group", Path: appPath + "." + group.Name, Var: v})
			}

			for _, cmd := range group.Commands {
				for _, v := range cmd.Vars {
					r = append(r, ScopedVar{Scope: "command", Path: appPath + "." + group.Name + "." + cmd.Name, Var: v})
				}
			}
		}

		for _, cmd := range app.Commands {
			for _, v := range cmd.Vars {
				r = append(r, ScopedVar{Scope: "command", Path: appPath + "." + cmd.Name, Var: v})
			}
		}
	}
	return r
}
