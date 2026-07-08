package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/quonaro/lota/logger"
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

var groupFields = []string{"desc", "dir", "color", "inherit_color", "show", "vars", "args", "shell", "log"}
var commandFields = []string{"desc", "dir", "color", "inherit_color", "show", "vars", "args", "script", "before", "after", "fallback", "finally", "depends", "parallel", "shell", "log", "native"}

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
		return nil, fmt.Errorf("log configuration must be a mapping (key-value) block at line %d, got %s", node.Line, nodeKindName(node.Kind))
	}

	var cfg LogConfig
	validLogFields := map[string]bool{"path": true, "truncate": true, "independent": true}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valueNode := node.Content[i+1]

		if !validLogFields[key] {
			suggestion := suggestField(key, []string{"path", "truncate", "independent"})
			if suggestion != "" {
				return nil, fmt.Errorf("unknown field %q in log %s at line %d. Did you mean: %s?", key, context, node.Content[i].Line, suggestion)
			}
			return nil, fmt.Errorf("unknown field %q in log %s at line %d", key, context, node.Content[i].Line)
		}

		switch key {
		case "path":
			cfg.Path = valueNode.Value
		case "truncate":
			var t bool
			if err := valueNode.Decode(&t); err != nil {
				return nil, fmt.Errorf("invalid truncate value in log %s at line %d: %w", context, valueNode.Line, err)
			}
			cfg.Truncate = t
		case "independent":
			var ind bool
			if err := valueNode.Decode(&ind); err != nil {
				return nil, fmt.Errorf("invalid independent value in log %s at line %d: %w", context, valueNode.Line, err)
			}
			if ind && !allowIndependent {
				return nil, fmt.Errorf("independent is not allowed")
			}
			cfg.Independent = ind
		}
	}

	if cfg.Path == "" {
		return nil, fmt.Errorf("log %s at line %d requires a path field", context, node.Line)
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
	return ParseConfigWithWriterAndImports(path, warnTo, true)
}

func ParseConfigWithWriterAndImports(path string, warnTo io.Writer, allowImports bool) (*AppConfig, error) {
	logger.Debugf("config: reading config from path: %s", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseConfigFromBytesWithWriterAndImports(data, warnTo, allowImports, path)
}

func ParseConfigFromBytes(data []byte) (*AppConfig, error) {
	return ParseConfigFromBytesWithWriter(data, os.Stderr)
}

func ParseConfigFromBytesWithWriter(data []byte, warnTo io.Writer) (*AppConfig, error) {
	return ParseConfigFromBytesWithWriterAndImports(data, warnTo, true, "")
}

func ParseConfigFromBytesWithWriterAndImports(data []byte, warnTo io.Writer, allowImports bool, basePath string) (*AppConfig, error) {
	return ParseConfigFromReaderWithWriterAndImports(bytes.NewReader(data), warnTo, allowImports, basePath)
}

func ParseConfigFromReader(r io.Reader) (*AppConfig, error) {
	return ParseConfigFromReaderWithWriter(r, os.Stderr)
}

func ParseConfigFromReaderWithWriter(r io.Reader, warnTo io.Writer) (*AppConfig, error) {
	return ParseConfigFromReaderWithWriterAndImports(r, warnTo, true, "")
}

func ParseConfigFromReaderWithWriterAndImports(r io.Reader, warnTo io.Writer, allowImports bool, basePath string) (*AppConfig, error) {
	logger.Debug("config: reading config from reader")
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var node yaml.Node
	logger.Debug("config: unmarshaling YAML")
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	// Unwrap Document node
	root := &node
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		root = node.Content[0]
	}

	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config file must be a mapping (key-value) block at line %d, got %s", root.Line, nodeKindName(root.Kind))
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
	logger.Debug("config: parsing root level fields")

	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i].Value
		valueNode := root.Content[i+1]

		switch key {
		case "vars":
			logger.Debug("config: parsing vars")
			if err := valueNode.Decode(&config.Vars); err != nil {
				return nil, fmt.Errorf("failed to parse vars at line %d: %w", valueNode.Line, err)
			}
		case "args":
			logger.Debug("config: parsing args")
			if err := valueNode.Decode(&config.RawArgs); err != nil {
				return nil, fmt.Errorf("failed to parse args at line %d: %w", valueNode.Line, err)
			}
			config.Args = make([]Arg, len(config.RawArgs))
			for j, arg := range config.RawArgs {
				if err := config.Args[j].Parse(arg); err != nil {
					return nil, fmt.Errorf("failed to parse arg %q at line %d: %w", arg, valueNode.Line, err)
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
		case "imports":
			if allowImports {
				if err := valueNode.Decode(&config.Imports); err != nil {
					return nil, fmt.Errorf("failed to parse imports at line %d: %w", valueNode.Line, err)
				}
			}
			// If allowImports is false, we silently ignore the imports field
		default:
			// Distinguish command (has "script" or "native" field) from group
			if hasField(valueNode, "script") || hasField(valueNode, "native") {
				logger.Debugf("config: parsing command: %s", key)
				var cmd Command
				cmd.Name = key
				if err := valueNode.Decode(&cmd); err != nil {
					return nil, fmt.Errorf("failed to parse command %q at line %d: %w", key, valueNode.Line, err)
				}
				config.Commands = append(config.Commands, cmd)
			} else {
				logger.Debugf("config: parsing group: %s", key)
				var group Group
				group.Name = key
				if err := valueNode.Decode(&group); err != nil {
					return nil, fmt.Errorf("failed to parse group %q at line %d: %w", key, valueNode.Line, err)
				}
				config.Groups = append(config.Groups, group)
			}
		}
	}

	logger.Debugf("config: parsed %d groups and %d commands", len(config.Groups), len(config.Commands))
	return config, nil
}

func ParseConfig(path string) (*AppConfig, error) {
	return ParseConfigWithWriter(path, os.Stderr)
}

// tryParseNestedCommandOrGroup attempts to parse a mapping node as a nested command or group.
// It returns true if the node was successfully parsed as either.
func (g *Group) tryParseNestedCommandOrGroup(key string, valueNode *yaml.Node, line int) (bool, error) {
	if valueNode.Kind != yaml.MappingNode {
		return false, nil
	}
	if hasField(valueNode, "script") || hasField(valueNode, "native") {
		var cmd Command
		if err := valueNode.Decode(&cmd); err != nil {
			return false, fmt.Errorf("failed to parse command %q in group %q at line %d: %w", key, g.Name, line, err)
		}
		cmd.Name = key
		g.Commands = append(g.Commands, cmd)
		return true, nil
	}
	var sub Group
	if err := valueNode.Decode(&sub); err != nil {
		return false, fmt.Errorf("failed to parse nested group %q in group %q at line %d: %w", key, g.Name, line, err)
	}
	sub.Name = key
	g.Groups = append(g.Groups, sub)
	return true, nil
}

func (g *Group) UnmarshalYAML(node *yaml.Node) error {
	logger.Debugf("config: unmarshaling group: %s", g.Name)
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("group %q must be a mapping (key-value) block at line %d, got %s", g.Name, node.Line, nodeKindName(node.Kind))
	}

	g.Commands = make([]Command, 0)
	g.Groups = make([]Group, 0)

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		valueNode := node.Content[i+1]
		switch key {
		case "desc":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			g.Desc = valueNode.Value
		case "dir":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			g.Dir = valueNode.Value
		case "color":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			g.Color = valueNode.Value
			if !isValidColor(g.Color) {
				return fmt.Errorf("invalid color %q for group %q at line %d. Available colors: %s", g.Color, g.Name, valueNode.Line, validColorsList())
			}
		case "inherit_color":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			var inherit bool
			if err := valueNode.Decode(&inherit); err != nil {
				return fmt.Errorf("invalid inherit_color value for group %q at line %d: %w", g.Name, valueNode.Line, err)
			}
			g.InheritColor = &inherit
		case "show":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			var show bool
			if err := valueNode.Decode(&show); err != nil {
				return fmt.Errorf("invalid show value for group %q at line %d: %w", g.Name, valueNode.Line, err)
			}
			g.Show = &show
		case "vars":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			if err := valueNode.Decode(&g.Vars); err != nil {
				return fmt.Errorf("failed to parse vars in group %q at line %d: %w", g.Name, valueNode.Line, err)
			}
		case "args":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			if err := valueNode.Decode(&g.RawArgs); err != nil {
				return fmt.Errorf("failed to parse args in group %q at line %d: %w", g.Name, valueNode.Line, err)
			}
			g.Args = make([]Arg, len(g.RawArgs))
			for j, arg := range g.RawArgs {
				if err := g.Args[j].Parse(arg); err != nil {
					return fmt.Errorf("invalid arg %q in group %q at line %d: %w", arg, g.Name, valueNode.Line, err)
				}
			}
		case "shell":
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			g.Shell = valueNode.Value
		case "log":
			logCfg, err := parseLogConfig(valueNode, true, fmt.Sprintf("in group %q", g.Name))
			if err != nil {
				return err
			}
			g.Log = logCfg
		default:
			if ok, err := g.tryParseNestedCommandOrGroup(key, valueNode, node.Content[i].Line); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
			if valueNode.Kind != yaml.MappingNode {
				suggestion := suggestField(key, groupFields)
				if suggestion != "" {
					return fmt.Errorf("unknown field %q in group %q at line %d. Did you mean: %s?", key, g.Name, node.Content[i].Line, suggestion)
				}
				return fmt.Errorf("unknown field %q in group %q at line %d (expected mapping for nested group, got %s)",
					key, g.Name, node.Content[i].Line, nodeKindName(valueNode.Kind))
			}
			var sub Group
			if err := valueNode.Decode(&sub); err != nil {
				return fmt.Errorf("failed to parse nested group %q in group %q at line %d: %w", key, g.Name, node.Content[i].Line, err)
			}
			sub.Name = key
			g.Groups = append(g.Groups, sub)
		}
	}

	return nil
}

func (c *Command) UnmarshalYAML(node *yaml.Node) error {
	logger.Debugf("config: unmarshaling command: %s", c.Name)
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("command %q must be a mapping (key-value) block at line %d, got %s", c.Name, node.Line, nodeKindName(node.Kind))
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
				return fmt.Errorf("invalid color %q for command %q at line %d. Available colors: %s", c.Color, c.Name, valueNode.Line, validColorsList())
			}
		case "inherit_color":
			var inherit bool
			if err := valueNode.Decode(&inherit); err != nil {
				return fmt.Errorf("invalid inherit_color value for command %q at line %d: %w", c.Name, valueNode.Line, err)
			}
			c.InheritColor = &inherit
		case "show":
			var show bool
			if err := valueNode.Decode(&show); err != nil {
				return fmt.Errorf("invalid show value for command %q at line %d: %w", c.Name, valueNode.Line, err)
			}
			c.Show = &show
		case "vars":
			if err := valueNode.Decode(&c.Vars); err != nil {
				return fmt.Errorf("failed to parse vars in command %q at line %d: %w", c.Name, valueNode.Line, err)
			}
		case "args":
			if err := valueNode.Decode(&c.RawArgs); err != nil {
				return fmt.Errorf("failed to parse args in command %q at line %d: %w", c.Name, valueNode.Line, err)
			}
			c.Args = make([]Arg, len(c.RawArgs))
			for j, arg := range c.RawArgs {
				if err := c.Args[j].Parse(arg); err != nil {
					return fmt.Errorf("invalid arg %q in command %q at line %d: %w", arg, c.Name, valueNode.Line, err)
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
		case "native":
			var native bool
			if err := valueNode.Decode(&native); err != nil {
				return fmt.Errorf("invalid native value for command %q at line %d: %w", c.Name, valueNode.Line, err)
			}
			c.Native = native
		case "depends":
			if err := valueNode.Decode(&c.Depends); err != nil {
				return fmt.Errorf("failed to parse depends in command %q at line %d: %w", c.Name, valueNode.Line, err)
			}
		case "parallel":
			var parallel bool
			if err := valueNode.Decode(&parallel); err != nil {
				return fmt.Errorf("invalid parallel value for command %q at line %d: %w", c.Name, valueNode.Line, err)
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
				return fmt.Errorf("unknown field %q in command %q at line %d. Did you mean: %s?", key, c.Name, node.Content[i].Line, suggestion)
			}
			return fmt.Errorf("unknown field %q in command %q at line %d", key, c.Name, node.Content[i].Line)
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

	// Optional marker: str? means the argument is not required
	if strings.HasSuffix(typeStr, "?") {
		a.Required = false
		typeStr = strings.TrimSuffix(typeStr, "?")
	} else {
		a.Required = true
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

	// A default value makes the argument optional
	if a.Default != "" {
		a.Required = false
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
