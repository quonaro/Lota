package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// loadYAMLFile loads a YAML file and flattens its structure into dot-notation keys.
// If prefix is provided, all keys will be prefixed with "prefix."
// Supports selecting a subsection via "path@section" syntax (e.g., "env.yml@public").
func loadYAMLFile(basePath, path, prefix string) (map[string]string, error) {
	// Parse path@section syntax
	section := ""
	filePath := path
	if idx := strings.LastIndex(path, "@"); idx != -1 {
		filePath = path[:idx]
		section = path[idx+1:]
	}

	fullPath := filePath
	if !filepath.IsAbs(filePath) {
		fullPath = filepath.Join(filepath.Dir(basePath), filePath)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file %s: %w", fullPath, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid YAML syntax in file %s: %w", fullPath, err)
	}

	// Unwrap Document node
	node := &root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		node = root.Content[0]
	}

	// If section is specified, navigate to it
	if section != "" {
		sectionNode, err := findSection(node, section)
		if err != nil {
			return nil, err
		}
		node = sectionNode
	}

	result := make(map[string]string)
	if err := flattenYAMLNode(node, prefix, result); err != nil {
		return nil, err
	}

	return result, nil
}

// findSection navigates to a subsection in the YAML tree using dot notation (e.g., "public", "database.connection").
func findSection(node *yaml.Node, section string) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("section %q must be a mapping (key-value) block, not %s", section, nodeKindName(node.Kind))
	}

	parts := strings.Split(section, ".")
	current := node

	for _, part := range parts {
		found := false
		for i := 0; i < len(current.Content); i += 2 {
			key := current.Content[i].Value
			if key == part {
				current = current.Content[i+1]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("section %q not found in file", section)
		}
	}

	return current, nil
}

// flattenYAMLNode recursively flattens a YAML node into dot-notation keys.
func flattenYAMLNode(node *yaml.Node, prefix string, result map[string]string) error {
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i].Value
			valueNode := node.Content[i+1]

			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}

			if err := flattenYAMLNode(valueNode, newPrefix, result); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			key := strconv.Itoa(i)
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}

			if err := flattenYAMLNode(item, newPrefix, result); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		result[prefix] = node.Value
	default:
		return fmt.Errorf("unsupported YAML structure: %s", nodeKindName(node.Kind))
	}

	return nil
}
