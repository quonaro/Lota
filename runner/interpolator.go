package runner

import (
	"fmt"
	"lota/config"
	"regexp"
	"strings"
	"strconv"
)

var placeholderRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

const defaultMaxArrayElements = 5

// ValidationError represents an interpolation validation error
type ValidationError struct {
	Placeholder string
	Reason      string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for placeholder '%s': %s", e.Placeholder, e.Reason)
}

// InterpolationContext holds all information needed for interpolation
type InterpolationContext struct {
	Vars    map[string]string
	Args    map[string]string
	ArgDefs []config.Arg // Argument definitions for type-aware interpolation
}

// Interpolate replaces variable and argument placeholders in script with their values.
// Supports type-aware interpolation and validation.
func Interpolate(script string, context InterpolationContext) (string, error) {
	result := script

	placeholders := findPlaceholders(script)

	for _, placeholder := range placeholders {
		value, err := interpolatePlaceholder(placeholder, context)
		if err != nil {
			return "", err
		}
		result = strings.ReplaceAll(result, "{{"+placeholder+"}}", value)
	}

	return result, nil
}

// InterpolateSimple is a simplified version for backward compatibility.
// On error, silently skips unresolved placeholders instead of failing.
func InterpolateSimple(script string, vars map[string]string, args map[string]string) string {
	context := InterpolationContext{
		Vars:    vars,
		Args:    args,
		ArgDefs: []config.Arg{},
	}

	result, err := Interpolate(script, context)
	if err != nil {
		result = script
		for name, value := range vars {
			result = strings.ReplaceAll(result, "{{"+name+"}}", value)
		}
		for name, value := range args {
			result = strings.ReplaceAll(result, "{{"+name+"}}", value)
		}
	}

	return result
}

// findPlaceholders extracts all {{placeholder}} patterns from script
func findPlaceholders(script string) []string {
	matches := placeholderRegex.FindAllStringSubmatch(script, -1)

	placeholders := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
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
		Reason:      "variable or argument not found",
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
