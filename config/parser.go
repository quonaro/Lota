package config

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var validColors = map[string]struct{}{
	"black": {}, "red": {}, "green": {}, "yellow": {}, "blue": {}, "magenta": {}, "cyan": {}, "white": {},
	"hiblack": {}, "hired": {}, "higreen": {}, "hiyellow": {}, "hiblue": {}, "himagenta": {}, "hicyan": {}, "hiwhite": {},
}

var reservedSystemVars = map[string]bool{
	"PATH":       true,
	"HOME":       true,
	"USER":       true,
	"SHELL":      true,
	"LANG":       true,
	"LC_ALL":     true,
	"TERM":       true,
	"PWD":        true,
	"OLDPWD":     true,
	"HOSTNAME":   true,
	"LOGNAME":    true,
	"MAIL":       true,
	"TMPDIR":     true,
	"DISPLAY":    true,
	"XAUTHORITY": true,
	"EDITOR":     true,
	"VISUAL":     true,
	"PAGER":      true,
}

func isValidColor(c string) bool {
	if c == "" {
		return true
	}
	_, ok := validColors[strings.ToLower(c)]
	if ok {
		return true
	}
	if len(c) == 7 && c[0] == '#' {
		_, err := strconv.ParseUint(c[1:], 16, 32)
		return err == nil
	}
	return false
}

func validColorsList() string {
	colors := make([]string, 0, len(validColors))
	for c := range validColors {
		colors = append(colors, c)
	}
	sort.Strings(colors)
	return strings.Join(colors, ", ") + " (or any #RRGGBB hex value)"
}

func nodeKindName(kind yaml.Kind) string {
	switch kind {
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.DocumentNode:
		return "document"
	case yaml.AliasNode:
		return "alias"
	default:
		return fmt.Sprintf("node(%d)", kind)
	}
}

// hasField checks if a mapping node has a key with the given name
func hasField(node *yaml.Node, field string) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == field {
			return true
		}
	}
	return false
}

var groupFields = []string{"desc", "dir", "color", "inherit_color", "vars", "args", "shell", "log"}
var commandFields = []string{"desc", "dir", "color", "inherit_color", "vars", "args", "script", "before", "after", "fallback", "finally", "depends", "parallel", "shell", "log"}

func suggestField(unknown string, valid []string) string {
	best := ""
	bestScore := 9999
	for _, v := range valid {
		dist := levenshteinDistance(unknown, v)
		if dist < bestScore {
			bestScore = dist
			best = v
		}
	}
	maxLen := max(len(unknown), len(best))
	if maxLen == 0 {
		return ""
	}
	normalized := float64(bestScore) / float64(maxLen)
	if normalized <= 0.5 {
		return best
	}
	return ""
}

func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := 0; j <= len(b); j++ {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			insertion := curr[j-1] + 1
			deletion := prev[j] + 1
			substitution := prev[j-1] + cost
			curr[j] = minInt(insertion, deletion, substitution)
		}
		prev, curr = curr, prev
	}

	return prev[len(b)]
}

func minInt(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func parseLogConfig(node *yaml.Node, allowIndependent bool, context string) (*LogConfig, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%d: expected mapping node for log, got %s", node.Line, nodeKindName(node.Kind))
	}

	var cfg LogConfig
	validLogFields := map[string]bool{"path": true, "truncate": true, "independent": true}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valueNode := node.Content[i+1]

		if !validLogFields[key] {
			suggestion := suggestField(key, []string{"path", "truncate", "independent"})
			if suggestion != "" {
				return nil, fmt.Errorf("%d: unknown field %q in log %s\nDid you mean: %s?", node.Content[i].Line, key, context, suggestion)
			}
			return nil, fmt.Errorf("%d: unknown field %q in log %s", node.Content[i].Line, key, context)
		}

		switch key {
		case "path":
			cfg.Path = valueNode.Value
		case "truncate":
			var t bool
			if err := valueNode.Decode(&t); err != nil {
				return nil, fmt.Errorf("%d: invalid truncate in log %s: %w", valueNode.Line, context, err)
			}
			cfg.Truncate = t
		case "independent":
			var ind bool
			if err := valueNode.Decode(&ind); err != nil {
				return nil, fmt.Errorf("%d: invalid independent in log %s: %w", valueNode.Line, context, err)
			}
			if ind && !allowIndependent {
				return nil, fmt.Errorf("%d: independent is not allowed in root-level log", valueNode.Line)
			}
			cfg.Independent = ind
		}
	}

	if cfg.Path == "" {
		return nil, fmt.Errorf("%d: log %s requires a path", node.Line, context)
	}

	return &cfg, nil
}

func normalizeImportTags(node *yaml.Node) []string {
	seen := make(map[string]struct{})
	var deprecated []string

	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && strings.HasPrefix(n.Tag, "!import:") {
			if _, ok := seen[n.Tag]; !ok {
				seen[n.Tag] = struct{}{}
				deprecated = append(deprecated, n.Tag)
			}
			n.Tag = strings.TrimPrefix(n.Tag, "!")
		}
		for _, child := range n.Content {
			walk(child)
		}
	}
	walk(node)
	return deprecated
}

func ParseConfigWithWriter(path string, warnTo io.Writer) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	// Unwrap Document node
	root := &node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		root = node.Content[0]
	}

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%d: expected mapping node, got %s", root.Line, nodeKindName(root.Kind))
	}

	deprecatedTags := normalizeImportTags(root)
	for _, tag := range deprecatedTags {
		if warnTo != nil {
			_, _ = fmt.Fprintf(warnTo, "\033[33mwarning: %s syntax is deprecated, use %s instead\033[0m\n", tag, strings.TrimPrefix(tag, "!"))
		}
	}

	config := &AppConfig{
		Groups:   make([]Group, 0),
		Commands: make([]Command, 0),
	}

	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i].Value
		valueNode := root.Content[i+1]

		switch key {
		case "vars":
			if err := valueNode.Decode(&config.Vars); err != nil {
				return nil, err
			}
		case "args":
			if err := valueNode.Decode(&config.RawArgs); err != nil {
				return nil, err
			}
			config.Args = make([]Arg, len(config.RawArgs))
			for j, arg := range config.RawArgs {
				if err := config.Args[j].Parse(arg); err != nil {
					return nil, err
				}
			}
		case "shell":
			config.Shell = valueNode.Value
		case "log":
			logCfg, err := parseLogConfig(valueNode, false, "at app level")
			if err != nil {
				return nil, err
			}
			config.Log = logCfg
		default:
			// Distinguish command (has "script" field) from group
			if hasField(valueNode, "script") {
				var cmd Command
				cmd.Name = key
				if err := valueNode.Decode(&cmd); err != nil {
					return nil, err
				}
				config.Commands = append(config.Commands, cmd)
			} else {
				var group Group
				group.Name = key
				if err := valueNode.Decode(&group); err != nil {
					return nil, err
				}
				config.Groups = append(config.Groups, group)
			}
		}
	}

	return config, nil
}

func ParseConfig(path string) (*AppConfig, error) {
	return ParseConfigWithWriter(path, os.Stderr)
}

func (g *Group) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%d: expected mapping node for group, got %s", node.Line, nodeKindName(node.Kind))
	}

	g.Commands = make([]Command, 0)
	g.Groups = make([]Group, 0)

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valueNode := node.Content[i+1]
		switch key {
		case "desc":
			g.Desc = valueNode.Value
		case "dir":
			g.Dir = valueNode.Value
		case "color":
			g.Color = valueNode.Value
			if !isValidColor(g.Color) {
				return fmt.Errorf("%d: invalid color %q for group %q\nAvailable colors: %s", valueNode.Line, g.Color, g.Name, validColorsList())
			}
		case "inherit_color":
			var inherit bool
			if err := valueNode.Decode(&inherit); err != nil {
				return fmt.Errorf("%d: invalid inherit_color for group %q: %w", valueNode.Line, g.Name, err)
			}
			g.InheritColor = &inherit
		case "vars":
			if err := valueNode.Decode(&g.Vars); err != nil {
				return fmt.Errorf("%d: error parsing vars in group %q: %w", valueNode.Line, g.Name, err)
			}
		case "args":
			if err := valueNode.Decode(&g.RawArgs); err != nil {
				return fmt.Errorf("%d: error parsing args in group %q: %w", valueNode.Line, g.Name, err)
			}
			g.Args = make([]Arg, len(g.RawArgs))
			for j, arg := range g.RawArgs {
				if err := g.Args[j].Parse(arg); err != nil {
					return fmt.Errorf("%d: invalid arg %q in group %q: %w", valueNode.Line, arg, g.Name, err)
				}
			}
		case "log":
			logCfg, err := parseLogConfig(valueNode, true, fmt.Sprintf("in group %q", g.Name))
			if err != nil {
				return err
			}
			g.Log = logCfg
		default:
			if hasField(valueNode, "script") {
				var cmd Command
				if err := valueNode.Decode(&cmd); err != nil {
					return fmt.Errorf("%d: error parsing command %q in group %q: %w", node.Content[i].Line, key, g.Name, err)
				}
				cmd.Name = key
				g.Commands = append(g.Commands, cmd)
			} else {
				if valueNode.Kind != yaml.MappingNode {
					suggestion := suggestField(key, groupFields)
					if suggestion != "" {
						return fmt.Errorf("%d: unknown field %q in group %q\nDid you mean: %s?", node.Content[i].Line, key, g.Name, suggestion)
					}
					return fmt.Errorf("%d: unknown field %q in group %q (expected mapping for nested group, got %s)",
						node.Content[i].Line, key, g.Name, nodeKindName(valueNode.Kind))
				}
				var sub Group
				if err := valueNode.Decode(&sub); err != nil {
					return fmt.Errorf("%d: error parsing nested group %q in group %q: %w", node.Content[i].Line, key, g.Name, err)
				}
				sub.Name = key
				g.Groups = append(g.Groups, sub)
			}
		}
	}

	return nil
}

func (c *Command) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("%d: expected mapping node for command, got %s", node.Line, nodeKindName(node.Kind))
	}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valueNode := node.Content[i+1]
		switch key {
		case "desc":
			c.Desc = valueNode.Value
		case "dir":
			c.Dir = valueNode.Value
		case "color":
			c.Color = valueNode.Value
			if !isValidColor(c.Color) {
				return fmt.Errorf("%d: invalid color %q for command %q\nAvailable colors: %s", valueNode.Line, c.Color, c.Name, validColorsList())
			}
		case "inherit_color":
			var inherit bool
			if err := valueNode.Decode(&inherit); err != nil {
				return fmt.Errorf("%d: invalid inherit_color for command %q: %w", valueNode.Line, c.Name, err)
			}
			c.InheritColor = &inherit
		case "vars":
			if err := valueNode.Decode(&c.Vars); err != nil {
				return fmt.Errorf("%d: error parsing vars in command %q: %w", valueNode.Line, c.Name, err)
			}
		case "args":
			if err := valueNode.Decode(&c.RawArgs); err != nil {
				return fmt.Errorf("%d: error parsing args in command %q: %w", valueNode.Line, c.Name, err)
			}
			c.Args = make([]Arg, len(c.RawArgs))
			for j, arg := range c.RawArgs {
				if err := c.Args[j].Parse(arg); err != nil {
					return fmt.Errorf("%d: invalid arg %q in command %q: %w", valueNode.Line, arg, c.Name, err)
				}
			}
		case "script":
			c.Script = valueNode.Value
		case "before":
			c.Before = valueNode.Value
		case "after":
			c.After = valueNode.Value
		case "fallback":
			c.Fallback = valueNode.Value
		case "finally":
			c.Finally = valueNode.Value
		case "depends":
			if err := valueNode.Decode(&c.Depends); err != nil {
				return fmt.Errorf("%d: error parsing depends in command %q: %w", valueNode.Line, c.Name, err)
			}
		case "parallel":
			var parallel bool
			if err := valueNode.Decode(&parallel); err != nil {
				return fmt.Errorf("%d: invalid parallel value in command %q: %w", valueNode.Line, c.Name, err)
			}
			c.Parallel = &parallel
		case "log":
			logCfg, err := parseLogConfig(valueNode, true, fmt.Sprintf("in command %q", c.Name))
			if err != nil {
				return err
			}
			c.Log = logCfg
		default:
			suggestion := suggestField(key, commandFields)
			if suggestion != "" {
				return fmt.Errorf("%d: unknown field %q in command %q\nDid you mean: %s?", node.Content[i].Line, key, c.Name, suggestion)
			}
			return fmt.Errorf("%d: unknown field %q in command %q", node.Content[i].Line, key, c.Name)
		}
	}

	return nil
}

func (a *Arg) Parse(s string) error {
	// Wildcard: ...args
	if strings.HasPrefix(s, "...") {
		a.Wildcard = true
		a.Name = strings.TrimPrefix(s, "...")
		return nil
	}

	// Formats:
	// name | name|short | name:type | name|short:type | name:type=default | name|short:type=default
	parts := strings.SplitN(s, ":", 2)

	nameParts := strings.Split(parts[0], "|")
	a.Name = nameParts[0]
	if len(nameParts) > 1 {
		a.Short = nameParts[1]
	}

	if len(parts) == 1 {
		return nil
	}

	// Parse type and optional default
	typeParts := strings.SplitN(parts[1], "=", 2)
	typeStr := typeParts[0]
	if len(typeParts) > 1 {
		a.Default = typeParts[1]
	}

	// Parse arr[N]
	if strings.HasPrefix(typeStr, "arr[") {
		a.Type = "arr"
		numStr := strings.TrimPrefix(typeStr, "arr[")
		numStr = strings.TrimSuffix(numStr, "]")
		if numStr != "" {
			if num, err := strconv.Atoi(numStr); err == nil {
				a.MaxArr = &num
			}
		}
	} else {
		a.Type = typeStr
	}

	return nil
}

func (a *Arg) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: expected scalar node for arg, got %s", node.Line, nodeKindName(node.Kind))
	}
	return a.Parse(node.Value)
}

func (v *Var) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: expected scalar node for var, got %s", node.Line, nodeKindName(node.Kind))
	}
	v.IsFile = false
	tag := node.Tag
	value := node.Value

	// Format: import:format <path> [prefix] (new)
	// Format: !import:format <path> [prefix] (deprecated)
	isImport := strings.HasPrefix(tag, "import:")
	isDeprecated := strings.HasPrefix(tag, "!import:")

	if isImport || isDeprecated {
		v.IsFile = true
		if isDeprecated {
			v.Format = strings.TrimPrefix(tag, "!import:")
		} else {
			v.Format = strings.TrimPrefix(tag, "import:")
		}

		// Parse: path [prefix]
		fields := strings.Fields(strings.TrimSpace(value))
		if len(fields) == 0 {
			return fmt.Errorf("line %d: import requires a file path", node.Line)
		}

		v.FromFile = fields[0]
		if len(fields) > 1 {
			v.Prefix = fields[1]
		}
		return nil
	}

	// Also support inline syntax without YAML tag: "import:env path prefix"
	if strings.HasPrefix(value, "import:") {
		v.IsFile = true
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return fmt.Errorf("line %d: import requires a file path", node.Line)
		}
		formatParts := strings.SplitN(fields[0], ":", 2)
		if len(formatParts) != 2 {
			return fmt.Errorf("line %d: invalid import format %q", node.Line, fields[0])
		}
		v.Format = formatParts[1]
		if len(fields) > 1 {
			v.FromFile = fields[1]
		}
		if len(fields) > 2 {
			v.Prefix = fields[2]
		}
		return nil
	}

	// Format: name=value
	parts := strings.SplitN(value, "=", 2)
	if len(parts) == 2 {
		v.Name, v.Value = parts[0], parts[1]
	} else {
		v.Name, v.Value = parts[0], ""
	}

	// Validate against reserved system variable names
	if reservedSystemVars[v.Name] {
		return fmt.Errorf("line %d: variable name %q is reserved for system use and cannot be overridden", node.Line, v.Name)
	}

	return nil
}
