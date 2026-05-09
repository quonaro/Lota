package runner

import (
	"fmt"
	"lota/config"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var placeholderRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

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
	Vars    map[string]string
	Args    map[string]string
	ArgDefs []config.Arg // Argument definitions for type-aware interpolation
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
	result := script

	placeholders := findPlaceholders(script)

	// Collect all validation errors
	var errors []string
	for _, placeholder := range placeholders {
		value, err := interpolatePlaceholder(placeholder, context)
		if err != nil {
			similar := findSimilarVars(placeholder, context.Vars)
			if len(similar) > 0 {
				errors = append(errors, fmt.Sprintf("%s not found. Available vars with '%s': %s", placeholder, placeholder, strings.Join(similar, ", ")))
			} else {
				errors = append(errors, fmt.Sprintf("%s is required", placeholder))
			}
			continue
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

// interpolatePlaceholder interpolates a single placeholder.
// vars have higher priority than args: same name in both — var wins.
func interpolatePlaceholder(placeholder string, context InterpolationContext) (string, error) {
	if value, exists := context.Vars[placeholder]; exists {
		return value, nil
	}

	if value, exists := context.Args[placeholder]; exists {
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
