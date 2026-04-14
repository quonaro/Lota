package runner

import (
	"fmt"
	"lota/config"
	"strings"
)

const defaultMaxArrayElements = 5

// ParseArgs converts command line arguments to a map based on argument definitions.
// Supports wildcard handling, boolean flags, and validation.
func ParseArgs(cliArgs []string, argDefs []config.Arg) (map[string]string, error) {
	result := make(map[string]string)

	// Build lookup maps for flags
	flagToArgDef := make(map[string]*config.Arg)
	positionalArgs := make([]*config.Arg, 0)
	var wildcardArg *config.Arg

	for i := range argDefs {
		argDef := &argDefs[i]
		if argDef.Wildcard {
			wildcardArg = argDef
		} else {
			// bool args are always flags (--name or --!name).
			// str/int/arr args with default values are flags.
			// str/int/arr args with Short are flags.
			// str/int/arr args without Short and without default are positional.
			isFlag := argDef.Short != "" || argDef.Type == "bool" || argDef.Default != ""

			if isFlag {
				if argDef.Name != "" {
					flagToArgDef["--"+argDef.Name] = argDef
				}
				if argDef.Short != "" {
					flagToArgDef["-"+argDef.Short] = argDef
				}
			} else {
				positionalArgs = append(positionalArgs, argDef)
			}
		}
	}

	i := 0
	positionalIndex := 0

	for i < len(cliArgs) {
		arg := cliArgs[i]

		// -- signals end of flags; collect everything remaining into wildcard
		if arg == "--" {
			i++
			if wildcardArg == nil {
				return nil, fmt.Errorf("unexpected argument separator '--'")
			}
			wildcardValues := make([]string, 0, len(cliArgs)-i)
			for i < len(cliArgs) {
				wildcardValues = append(wildcardValues, cliArgs[i])
				i++
			}
			result[wildcardArg.Name] = strings.Join(wildcardValues, " ")
			break
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			flagName := arg
			var value string
			hasValue := false

			// Handle --name=value format
			if strings.Contains(arg, "=") {
				parts := strings.SplitN(arg, "=", 2)
				flagName = parts[0]
				value = parts[1]
				hasValue = true
			}

			var matchingArgDef *config.Arg

			if argDef, exists := flagToArgDef[flagName]; exists {
				matchingArgDef = argDef
			} else if !hasValue && strings.HasPrefix(flagName, "--!") {
				negatedName := "--" + strings.TrimPrefix(flagName, "--!")
				if argDef, exists := flagToArgDef[negatedName]; exists {
					matchingArgDef = argDef
					value = "false"
					hasValue = true
				}
			}

			if matchingArgDef != nil {
				switch matchingArgDef.Type {
				case "bool":
					if hasValue {
						boolValue, err := parseBoolValue(value)
						if err != nil {
							return nil, fmt.Errorf("invalid boolean value for %s: %v", matchingArgDef.Name, err)
						}
						result[matchingArgDef.Name] = boolValue
					} else {
						result[matchingArgDef.Name] = "true"
					}
					i++
				default:
					if hasValue {
						result[matchingArgDef.Name] = value
						i++
					} else {
						if i+1 >= len(cliArgs) {
							return nil, fmt.Errorf("flag %s requires a value", arg)
						}
						result[matchingArgDef.Name] = cliArgs[i+1]
						i += 2
					}
				}
			} else {
				return nil, fmt.Errorf("unknown flag: %s", arg)
			}

		} else {
			switch {
			case wildcardArg != nil && positionalIndex >= len(positionalArgs):
				// Collect remaining for wildcard
				wildcardValues := make([]string, 0)
				for i < len(cliArgs) {
					if strings.HasPrefix(cliArgs[i], "-") && len(cliArgs[i]) > 1 {
						break
					}
					wildcardValues = append(wildcardValues, cliArgs[i])
					i++
				}
				result[wildcardArg.Name] = strings.Join(wildcardValues, " ")
			case positionalIndex < len(positionalArgs):
				argDef := positionalArgs[positionalIndex]

				switch argDef.Type {
				case "arr":
					arrayArgs := make([]string, 0)
					maxElements := defaultMaxArrayElements
					if argDef.MaxArr != nil {
						maxElements = *argDef.MaxArr
					}
					for i < len(cliArgs) && len(arrayArgs) < maxElements {
						if strings.HasPrefix(cliArgs[i], "-") && len(cliArgs[i]) > 1 {
							break
						}
						arrayArgs = append(arrayArgs, cliArgs[i])
						i++
					}
					result[argDef.Name] = strings.Join(arrayArgs, ",")
					positionalIndex++
				case "bool":
					boolValue, err := parseBoolValue(arg)
					if err != nil {
						return nil, fmt.Errorf("invalid boolean value for %s: %v", argDef.Name, err)
					}
					result[argDef.Name] = boolValue
					i++
					positionalIndex++
				default:
					result[argDef.Name] = arg
					i++
					positionalIndex++
				}
			default:
				return nil, fmt.Errorf("unexpected argument: %s", arg)
			}
		}
	}

	// Fill in defaults for missing arguments
	for _, argDef := range argDefs {
		if _, exists := result[argDef.Name]; !exists {
			if argDef.Default != "" {
				result[argDef.Name] = argDef.Default
			} else if argDef.Required && !argDef.Wildcard {
				return nil, fmt.Errorf("required argument %s is missing", argDef.Name)
			}
		}
	}

	return result, nil
}

// parseBoolValue parses boolean values with negation support
func parseBoolValue(value string) (string, error) {
	lower := strings.ToLower(value)

	switch lower {
	case "true", "1", "yes", "on":
		return "true", nil
	case "false", "0", "no", "off":
		return "false", nil
	default:
		if strings.HasPrefix(value, "!") {
			negated := strings.TrimPrefix(value, "!")
			switch strings.ToLower(negated) {
			case "true":
				return "false", nil
			case "false":
				return "true", nil
			default:
				return "false", nil
			}
		}
		return "", fmt.Errorf("invalid boolean value: %s", value)
	}
}
