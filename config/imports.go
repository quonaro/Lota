package config

import (
	"fmt"
)

// ProcessImports processes all imports in the config and merges them.
// It handles both local files and URLs, and supports namespacing.
func ProcessImports(cfg *AppConfig, basePath string) error {
	if len(cfg.Imports) == 0 {
		return nil
	}

	for _, imp := range cfg.Imports {
		if err := processImport(cfg, imp, basePath); err != nil {
			return fmt.Errorf("failed to process import %q: %w", imp.URL, err)
		}
	}

	return nil
}

// processImport processes a single import and merges it into the config.
func processImport(cfg *AppConfig, imp ImportConfig, basePath string) error {
	// Resolve the import path
	resolvedPath := ResolveImportPath(imp.URL, basePath)

	// If it's a URL, fetch it first
	if IsURL(resolvedPath) {
		_, err := FetchURL(resolvedPath)
		if err != nil {
			return fmt.Errorf("failed to fetch URL: %w", err)
		}
		// Use cache path for parsing
		resolvedPath = GetCachePath(resolvedPath)
	}

	// Parse the imported config WITHOUT allowing its own imports
	importedCfg, err := ParseConfigWithWriterAndImports(resolvedPath, nil, false)
	if err != nil {
		return fmt.Errorf("failed to parse imported config: %w", err)
	}

	// If namespace is specified, wrap the imported config in a group
	if imp.Namespace != "" {
		wrapInNamespace(cfg, importedCfg, imp.Namespace, resolvedPath)
	} else {
		// Merge directly into root
		mergeIntoRoot(cfg, importedCfg)
	}

	// Merge vars - always merge into root (no namespace for vars)
	cfg.Vars = append(cfg.Vars, importedCfg.Vars...)

	return nil
}

// wrapInNamespace wraps the imported config in a group with the given namespace.
func wrapInNamespace(cfg *AppConfig, importedCfg *AppConfig, namespace, importPath string) {
	// Create a group to hold the imported config
	namespaceGroup := Group{
		Name:     namespace,
		Desc:     fmt.Sprintf("Imported from %s", importPath),
		Groups:   importedCfg.Groups,
		Commands: importedCfg.Commands,
		Vars:     importedCfg.Vars,
		Shell:    importedCfg.Shell,
		Log:      importedCfg.Log,
	}

	// Add the namespace group to the config
	cfg.Groups = append(cfg.Groups, namespaceGroup)
}

// mergeIntoRoot merges the imported config directly into the root config.
// Local config properties override imported ones.
func mergeIntoRoot(cfg *AppConfig, importedCfg *AppConfig) {
	// Merge groups - local overrides imported
	cfg.Groups = mergeGroups(cfg.Groups, importedCfg.Groups)

	// Merge commands - local overrides imported
	cfg.Commands = mergeCommands(cfg.Commands, importedCfg.Commands)

	// Shell and log - local overrides imported
	if cfg.Shell == "" && importedCfg.Shell != "" {
		cfg.Shell = importedCfg.Shell
	}
	if cfg.Log == nil && importedCfg.Log != nil {
		cfg.Log = importedCfg.Log
	}
}

// mergeGroups merges two group slices, with local groups overriding imported ones.
func mergeGroups(local, imported []Group) []Group {
	// Create a map of local groups by name
	localMap := make(map[string]Group)
	for _, g := range local {
		localMap[g.Name] = g
	}

	// Add imported groups that don't exist locally
	for _, g := range imported {
		if _, exists := localMap[g.Name]; !exists {
			localMap[g.Name] = g
		}
	}

	// Convert back to slice
	result := make([]Group, 0, len(localMap))
	for _, g := range localMap {
		result = append(result, g)
	}
	return result
}

// mergeCommands merges two command slices, with local commands overriding imported ones.
func mergeCommands(local, imported []Command) []Command {
	// Create a map of local commands by name
	localMap := make(map[string]Command)
	for _, c := range local {
		localMap[c.Name] = c
	}

	// Add imported commands that don't exist locally
	for _, c := range imported {
		if _, exists := localMap[c.Name]; !exists {
			localMap[c.Name] = c
		}
	}

	// Convert back to slice
	result := make([]Command, 0, len(localMap))
	for _, c := range localMap {
		result = append(result, c)
	}
	return result
}
