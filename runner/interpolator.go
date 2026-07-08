package runner

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/quonaro/lota/config"
	"github.com/quonaro/lota/logger"
)

var placeholderRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)
var assignmentRegex = regexp.MustCompile(`(?m)(^|[;&])\s*(?:export\s+|local\s+|declare\s+|typeset\s+|readonly\s+)?([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:\+?=)`)
var loopVarRegex = regexp.MustCompile(`(?m)(^|[;{])\s*(?:for|select)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+in\b`)
var readCommandRegex = regexp.MustCompile(`\bread(?:\s+-[^\s]+)*((?:\s+[a-zA-Z_][a-zA-Z0-9_]*)+)`)

// ValidationError represents an interpolation validation error
type ValidationError struct {
	Placeholder string
	Reason      string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s is not set", e.Placeholder)
}

// InterpolationContext holds all information needed for interpolation
type InterpolationContext struct {
	Vars              map[string]string
	Args              map[string]string
	ArgDefs           []config.Arg    // Argument definitions for type-aware interpolation
	DeprecationWarned map[string]bool // Tracks which deprecation warnings have been shown
	WarnWriter        io.Writer       // Destination for deprecation warnings; nil suppresses them
}

// findSimilarVars finds variables with similar prefix to help users debug
func findSimilarVars(placeholder string, vars map[string]string) []string {
	// Extract prefix (first part before dot, or entire placeholder if no dot)
	prefix := placeholder
	if idx := strings.Index(placeholder, "."); idx != -1 {
		prefix = placeholder[:idx]
	}

	// Find all vars that start with the prefix
	var similar []string
	for name := range vars {
		if strings.HasPrefix(name, prefix+".") || name == prefix {
			similar = append(similar, name)
		}
	}

	// Sort for deterministic output
	sort.Strings(similar)
	return similar
}

// Interpolate replaces variable and argument placeholders in script with their values.
// Supports type-aware interpolation and validation.
func Interpolate(script string, context InterpolationContext) (string, error) {
	logger.Debugf("interpolator: starting interpolation")
	result := script
	localVars := detectScriptLocalVars(script)
	systemEnvVars := buildSystemEnvVarSet()
	logger.Debugf("interpolator: detected %d local vars, %d system env vars", len(localVars), len(systemEnvVars))

	// Process $var syntax first
	dollarVars := findDollarVars(script)
	logger.Debugf("interpolator: found %d $var patterns", len(dollarVars))
	for _, varName := range dollarVars {
		// Skip special variables like $CWD
		if isSpecialVariable(varName) {
			continue
		}
		value, err := interpolatePlaceholder(varName, context)
		if err != nil {
			if localVars[varName] || systemEnvVars[varName] {
				continue
			}
			similar := findSimilarVars(varName, context.Vars)
			if len(similar) > 0 {
				return "", fmt.Errorf("variable '%s' not found. Available variables with similar name: %s. Check --help for more information", varName, strings.Join(similar, ", "))
			}
			if argDefined(varName, context.ArgDefs) {
				return "", fmt.Errorf("argument '%s' is required. Check --help for more information", varName)
			}
			return "", fmt.Errorf("variable '%s' is required. Check --help for more information", varName)
		}
		result = strings.ReplaceAll(result, "$"+varName, value)
	}

	// Process {{}} syntax (deprecated for vars)
	placeholders := findPlaceholders(script)
	logger.Debugf("interpolator: found %d {{}} placeholders", len(placeholders))

	// Collect all validation errors
	var errors []string
	for _, placeholder := range placeholders {
		value, err := interpolatePlaceholder(placeholder, context)
		if err != nil {
			similar := findSimilarVars(placeholder, context.Vars)
			if len(similar) > 0 {
				errors = append(errors, fmt.Sprintf("variable '%s' not found. Available variables with similar name: %s", placeholder, strings.Join(similar, ", ")))
			} else {
				errors = append(errors, fmt.Sprintf("variable '%s' is required", placeholder))
			}
			continue
		}
		// Show deprecation warning for {{}} syntax
		if _, isArg := context.Args[placeholder]; isArg {
			if context.DeprecationWarned == nil {
				context.DeprecationWarned = make(map[string]bool)
			}
			if !context.DeprecationWarned[placeholder] {
				if context.WarnWriter != nil {
					_, _ = fmt.Fprintf(context.WarnWriter, "\033[33mwarning: {{%s}} interpolation is deprecated, use $%s instead\033[0m\n", placeholder, placeholder)
				}
				context.DeprecationWarned[placeholder] = true
			}
		} else if _, isVar := context.Vars[placeholder]; isVar {
			if context.DeprecationWarned == nil {
				context.DeprecationWarned = make(map[string]bool)
			}
			if !context.DeprecationWarned[placeholder] {
				if context.WarnWriter != nil {
					_, _ = fmt.Fprintf(context.WarnWriter, "\033[33mwarning: {{%s}} interpolation is deprecated, use $%s instead\033[0m\n", placeholder, placeholder)
				}
				context.DeprecationWarned[placeholder] = true
			}
		}
		result = strings.ReplaceAll(result, "{{"+placeholder+"}}", value)
	}

	if len(errors) > 0 {
		return "", fmt.Errorf("%s. Check --help for more information", strings.Join(errors, "; "))
	}

	return result, nil
}

// findPlaceholders extracts all unique {{placeholder}} patterns from script
func findPlaceholders(script string) []string {
	matches := placeholderRegex.FindAllStringSubmatch(script, -1)

	seen := make(map[string]bool)
	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			placeholders = append(placeholders, match[1])
		}
	}
	return placeholders
}

// findDollarVars extracts all unique $var patterns from script.
// It ignores $var inside single quotes, matching shell behavior.
func findDollarVars(script string) []string {
	seen := make(map[string]bool)
	vars := make([]string, 0)
	inSingleQuote := false
	inDoubleQuote := false
	escape := false

	for i := 0; i < len(script); i++ {
		ch := script[i]
		if escape {
			escape = false
			continue
		}
		if ch == '\\' && !inSingleQuote {
			escape = true
			continue
		}
		if ch == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if ch == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if ch == '$' && !inSingleQuote {
			if i+1 < len(script) && isShellIdentRune(rune(script[i+1]), true) {
				start := i + 1
				end := start + 1
				for end < len(script) && isShellIdentRune(rune(script[end]), false) {
					end++
				}
				name := script[start:end]
				// Handle dot notation: $cfg.app_name
				for end < len(script) && script[end] == '.' {
					dotStart := end + 1
					if dotStart < len(script) && isShellIdentRune(rune(script[dotStart]), true) {
						dotEnd := dotStart + 1
						for dotEnd < len(script) && isShellIdentRune(rune(script[dotEnd]), false) {
							dotEnd++
						}
						name = name + "." + script[dotStart:dotEnd]
						end = dotEnd
					} else {
						break
					}
				}
				if !seen[name] {
					seen[name] = true
					vars = append(vars, name)
				}
				i = end - 1
			}
		}
	}
	return vars
}

// isSpecialVariable checks if a variable name is a special system variable
func isSpecialVariable(name string) bool {
	specialVars := map[string]bool{
		"CWD": true,
	}
	return specialVars[name]
}

// argDefined returns true if placeholder is a declared argument name
func argDefined(name string, defs []config.Arg) bool {
	for _, def := range defs {
		if def.Name == name {
			return true
		}
	}
	return false
}

// detectScriptLocalVars extracts shell variables that are defined within the script
func detectScriptLocalVars(script string) map[string]bool {
	locals := make(map[string]bool)
	assignmentMatches := assignmentRegex.FindAllStringSubmatch(script, -1)
	for _, match := range assignmentMatches {
		if len(match) >= 3 {
			locals[match[2]] = true
		}
	}

	loopMatches := loopVarRegex.FindAllStringSubmatch(script, -1)
	for _, match := range loopMatches {
		if len(match) >= 3 {
			locals[match[2]] = true
		}
	}

	readMatches := readCommandRegex.FindAllStringSubmatch(script, -1)
	for _, match := range readMatches {
		if len(match) < 2 {
			continue
		}
		fields := strings.Fields(match[1])
		for _, field := range fields {
			if name, ok := extractIdentifier(field); ok {
				locals[name] = true
			}
		}
	}

	return locals
}

// buildSystemEnvVarSet collects current process environment variable names
func buildSystemEnvVarSet() map[string]bool {
	vars := make(map[string]bool)
	for _, env := range os.Environ() {
		if idx := strings.Index(env, "="); idx > 0 {
			name := env[:idx]
			if isValidShellIdentifier(name) {
				vars[name] = true
			}
		}
	}
	return vars
}

func isValidShellIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if !isShellIdentRune(r, i == 0) {
			return false
		}
	}
	return true
}

func extractIdentifier(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	var builder strings.Builder
	for i, r := range token {
		if isShellIdentRune(r, i == 0) {
			builder.WriteRune(r)
			continue
		}
		if i == 0 {
			return "", false
		}
		break
	}
	if builder.Len() == 0 {
		return "", false
	}
	return builder.String(), true
}

func isShellIdentRune(r rune, first bool) bool {
	if first {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_'
	}
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// interpolatePlaceholder interpolates a single placeholder.
// args have higher priority than vars: same name in both — arg wins.
func interpolatePlaceholder(placeholder string, context InterpolationContext) (string, error) {
	logger.Debugf("interpolator: interpolating placeholder: %s", placeholder)
	// Check args first (higher priority)
	if value, exists := context.Args[placeholder]; exists {
		logger.Debugf("interpolator: found in args: %s=%s", placeholder, value)
		var argDef *config.Arg
		for _, def := range context.ArgDefs {
			if def.Name == placeholder {
				argDef = &def
				break
			}
		}

		if argDef != nil {
			return interpolateTypedValue(placeholder, value, *argDef)
		}

		return value, nil
	}

	// Then check vars
	if value, exists := context.Vars[placeholder]; exists {
		return value, nil
	}

	// Optional args without a value resolve to empty string
	for _, def := range context.ArgDefs {
		if def.Name == placeholder && !def.Required {
			return "", nil
		}
	}

	return "", ValidationError{
		Placeholder: placeholder,
		Reason:      fmt.Sprintf("'%s' is not defined", placeholder),
	}
}

// interpolateTypedValue processes value based on argument type
func interpolateTypedValue(name, value string, argDef config.Arg) (string, error) {
	switch argDef.Type {
	case "int":
		return interpolateInt(name, value)
	case "bool":
		return interpolateBool(name, value)
	case "arr":
		return interpolateArray(value)
	case "str", "":
		return value, nil
	default:
		return "", ValidationError{
			Placeholder: argDef.Name,
			Reason:      fmt.Sprintf("unknown type '%s'", argDef.Type),
		}
	}
}

// trimQuotes removes surrounding double quotes from a value
func trimQuotes(value string) string {
	if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return strings.Trim(value, `"`)
	}
	return value
}

// interpolateInt validates and formats integer values
func interpolateInt(name, value string) (string, error) {
	if value == "" {
		return "0", nil
	}

	value = trimQuotes(value)

	if _, err := strconv.Atoi(value); err != nil {
		return "", ValidationError{
			Placeholder: name,
			Reason:      fmt.Sprintf("invalid integer value '%s'", value),
		}
	}

	return value, nil
}

// interpolateBool handles boolean values with negation support
func interpolateBool(name, value string) (string, error) {
	if value == "" {
		return "false", nil
	}

	value = trimQuotes(value)

	result, err := parseBoolValue(value)
	if err != nil {
		return "", ValidationError{
			Placeholder: name,
			Reason:      fmt.Sprintf("invalid boolean value '%s'", value),
		}
	}
	return result, nil
}

// interpolateArray formats array values
func interpolateArray(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	value = trimQuotes(value)

	// Array values are comma-separated, ensure proper formatting
	parts := strings.Split(value, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}

	return strings.Join(parts, " "), nil
}
