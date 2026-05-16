package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

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

func ParseConfig(path string) (*AppConfig, error) {
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
		return nil, fmt.Errorf("expected mapping node, got %d", root.Kind)
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
		default:
			// Distinguish command (has "script" field) from group
			if hasField(valueNode, "script") {
				var cmd Command
				if err := valueNode.Decode(&cmd); err != nil {
					return nil, err
				}
				cmd.Name = key
				config.Commands = append(config.Commands, cmd)
			} else {
				var group Group
				if err := valueNode.Decode(&group); err != nil {
					return nil, err
				}
				group.Name = key
				config.Groups = append(config.Groups, group)
			}
		}
	}

	return config, nil
}

func (g *Group) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node for group, got %d", node.Kind)
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
		case "vars":
			if err := valueNode.Decode(&g.Vars); err != nil {
				return err
			}
		case "args":
			if err := valueNode.Decode(&g.RawArgs); err != nil {
				return err
			}
			g.Args = make([]Arg, len(g.RawArgs))
			for j, arg := range g.RawArgs {
				if err := g.Args[j].Parse(arg); err != nil {
					return err
				}
			}
		default:
			if hasField(valueNode, "script") {
				var cmd Command
				if err := valueNode.Decode(&cmd); err != nil {
					return err
				}
				cmd.Name = key
				g.Commands = append(g.Commands, cmd)
			} else {
				var sub Group
				if err := valueNode.Decode(&sub); err != nil {
					return err
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
		return fmt.Errorf("expected mapping node for command, got %d", node.Kind)
	}

	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		switch key {
		case "desc":
			c.Desc = node.Content[i+1].Value
		case "dir":
			c.Dir = node.Content[i+1].Value
		case "vars":
			if err := node.Content[i+1].Decode(&c.Vars); err != nil {
				return err
			}
		case "args":
			if err := node.Content[i+1].Decode(&c.RawArgs); err != nil {
				return err
			}
			c.Args = make([]Arg, len(c.RawArgs))
			for j, arg := range c.RawArgs {
				if err := c.Args[j].Parse(arg); err != nil {
					return err
				}
			}
		case "script":
			c.Script = node.Content[i+1].Value
		case "before":
			c.Before = node.Content[i+1].Value
		case "after":
			c.After = node.Content[i+1].Value
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
		return fmt.Errorf("expected scalar node for arg, got %d", node.Kind)
	}
	return a.Parse(node.Value)
}

func (v *Var) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("expected scalar node for var, got %d", node.Kind)
	}
	v.IsFile = false
	tag := node.Tag
	value := node.Value

	// Format: !import:format <path> [prefix]
	if strings.HasPrefix(tag, "!import:") {
		v.IsFile = true
		v.Format = strings.TrimPrefix(tag, "!import:")

		// Parse: path [prefix]
		fields := strings.Fields(strings.TrimSpace(value))
		if len(fields) == 0 {
			return fmt.Errorf("import requires a file path")
		}

		v.FromFile = fields[0]
		if len(fields) > 1 {
			v.Prefix = fields[1]
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

	return nil
}
