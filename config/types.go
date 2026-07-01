package config

import (
	"fmt"
	"strings"
)

type LogConfig struct {
	Path        string `yaml:"path"`
	Truncate    bool   `yaml:"truncate,omitempty"`
	Independent bool   `yaml:"independent,omitempty"`
}

type AppConfig struct {
	Shell    string     `yaml:"shell,omitempty"`
	Vars     []Var      `yaml:"vars"`
	Args     []Arg      `yaml:"-"`
	RawArgs  []string   `yaml:"args"`
	Log      *LogConfig `yaml:"log,omitempty"`
	Groups   []Group    `yaml:"groups"`
	Commands []Command  `yaml:"commands"`

	// Indexes
	commandsMap map[string]*Command
	groupsMap   map[string]*Group
}

type Group struct {
	Shell        string `yaml:"shell,omitempty"`
	Dir          string `yaml:"dir,omitempty"`
	Name         string
	Desc         string     `yaml:"desc"`
	Color        string     `yaml:"color,omitempty"`
	InheritColor *bool      `yaml:"inherit_color,omitempty"`
	Show         *bool      `yaml:"show,omitempty"`
	Vars         []Var      `yaml:"vars"`
	Args         []Arg      `yaml:"-"`
	RawArgs      []string   `yaml:"args"`
	Log          *LogConfig `yaml:"log,omitempty"`
	Commands     []Command
	Groups       []Group

	// Indexes
	commandsMap map[string]*Command
	groupsMap   map[string]*Group
}

type Command struct {
	Shell        string `yaml:"shell,omitempty"`
	Dir          string `yaml:"dir,omitempty"`
	Name         string
	Desc         string     `yaml:"desc"`
	Color        string     `yaml:"color,omitempty"`
	InheritColor *bool      `yaml:"inherit_color,omitempty"`
	Show         *bool      `yaml:"show,omitempty"`
	Vars         []Var      `yaml:"vars"`
	Args         []Arg      `yaml:"-"`
	RawArgs      []string   `yaml:"args"`
	Log          *LogConfig `yaml:"log,omitempty"`
	Depends      []string   `yaml:"depends,omitempty"`
	Parallel     *bool      `yaml:"parallel,omitempty"`
	Script       string     `yaml:"script"`
	Before       string     `yaml:"before"`
	After        string     `yaml:"after"`
	Fallback     string     `yaml:"fallback"`
	Finally      string     `yaml:"finally"`
	Native       bool       `yaml:"native,omitempty"`
}

type Arg struct {
	Name     string
	Short    string
	Type     string
	Default  string
	Required bool
	Wildcard bool
	MaxArr   *int
}

type Var struct {
	Name     string
	Value    string
	FromFile string `yaml:"-"` // path to file to import variables from
	Format   string `yaml:"-"` // file format: env, yaml, json, toml, etc.
	IsFile   bool   `yaml:"-"` // whether this variable is loaded from a file
	Prefix   string `yaml:"-"` // prefix for imported variables (e.g., "public" for import:yaml file.yaml public)
}

type SearchResult struct {
	Exists  bool
	Command *Command // non-nil if result is a command
	Groups  []*Group // chain of groups from outermost to innermost
}

func (c *AppConfig) BuildIndexes() error {
	c.commandsMap = make(map[string]*Command)
	for i := range c.Commands {
		if c.Commands[i].Name == "" {
			return fmt.Errorf("command with empty name")
		}
		if _, exists := c.commandsMap[c.Commands[i].Name]; exists {
			return fmt.Errorf("duplicate command name: %s", c.Commands[i].Name)
		}
		c.commandsMap[c.Commands[i].Name] = &c.Commands[i]
	}

	c.groupsMap = make(map[string]*Group)
	for i := range c.Groups {
		if c.Groups[i].Name == "" {
			return fmt.Errorf("group with empty name")
		}
		if _, exists := c.groupsMap[c.Groups[i].Name]; exists {
			return fmt.Errorf("duplicate group name: %s", c.Groups[i].Name)
		}
		if err := c.Groups[i].BuildIndexes(); err != nil {
			return fmt.Errorf("group %s: %w", c.Groups[i].Name, err)
		}
		c.groupsMap[c.Groups[i].Name] = &c.Groups[i]
	}

	if err := c.validateUniqueCommandNames(); err != nil {
		return err
	}
	return nil
}

func (c *AppConfig) validateUniqueCommandNames() error {
	seen := make(map[string][]string)
	for i := range c.Commands {
		seen[c.Commands[i].Name] = append(seen[c.Commands[i].Name], c.Commands[i].Name)
	}
	for i := range c.Groups {
		c.Groups[i].collectCommandNames(c.Groups[i].Name, seen)
	}
	for name, paths := range seen {
		if len(paths) > 1 {
			return fmt.Errorf("duplicate command name %q at paths %s", name, strings.Join(paths, ", "))
		}
	}
	return nil
}

func (g *Group) collectCommandNames(groupPath string, seen map[string][]string) {
	for i := range g.Commands {
		path := groupPath + "." + g.Commands[i].Name
		seen[g.Commands[i].Name] = append(seen[g.Commands[i].Name], path)
	}
	for i := range g.Groups {
		g.Groups[i].collectCommandNames(groupPath+"."+g.Groups[i].Name, seen)
	}
}

func (g *Group) BuildIndexes() error {
	g.commandsMap = make(map[string]*Command)
	for i := range g.Commands {
		if g.Commands[i].Name == "" {
			return fmt.Errorf("command with empty name")
		}
		if _, exists := g.commandsMap[g.Commands[i].Name]; exists {
			return fmt.Errorf("duplicate command name: %s", g.Commands[i].Name)
		}
		g.commandsMap[g.Commands[i].Name] = &g.Commands[i]
	}

	g.groupsMap = make(map[string]*Group)
	for i := range g.Groups {
		if g.Groups[i].Name == "" {
			return fmt.Errorf("group with empty name")
		}
		if _, exists := g.groupsMap[g.Groups[i].Name]; exists {
			return fmt.Errorf("duplicate group name: %s", g.Groups[i].Name)
		}
		if err := g.Groups[i].BuildIndexes(); err != nil {
			return fmt.Errorf("group %s: %w", g.Groups[i].Name, err)
		}
		g.groupsMap[g.Groups[i].Name] = &g.Groups[i]
	}
	return nil
}

func (c *AppConfig) Find(name string) SearchResult {
	if cmd, found := c.commandsMap[name]; found {
		return SearchResult{Exists: true, Command: cmd}
	}

	if group, found := c.groupsMap[name]; found {
		return SearchResult{Exists: true, Groups: []*Group{group}}
	}

	return SearchResult{Exists: false}
}

func (g *Group) Find(name string) SearchResult {
	if cmd, found := g.commandsMap[name]; found {
		return SearchResult{Exists: true, Command: cmd}
	}

	if sub, found := g.groupsMap[name]; found {
		return SearchResult{Exists: true, Groups: []*Group{sub}}
	}

	return SearchResult{Exists: false}
}

func (c *AppConfig) AllVars() []Var {
	var all []Var

	// App level vars
	all = append(all, c.Vars...)

	// Collect from groups (including nested)
	for _, group := range c.Groups {
		all = append(all, group.allVarsRecursive()...)
	}

	// Collect from commands
	for _, cmd := range c.Commands {
		all = append(all, cmd.Vars...)
	}

	return all
}

func (g *Group) allVarsRecursive() []Var {
	var all []Var

	// Group level vars
	all = append(all, g.Vars...)

	// Nested groups
	for _, sub := range g.Groups {
		all = append(all, sub.allVarsRecursive()...)
	}

	// Commands in this group
	for _, cmd := range g.Commands {
		all = append(all, cmd.Vars...)
	}

	return all
}
