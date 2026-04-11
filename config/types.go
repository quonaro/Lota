package config

import "fmt"

type AppConfig struct {
	Shell    string    `yaml:"shell,omitempty"`
	Vars     []Var     `yaml:"vars"`
	Args     []Arg     `yaml:"-"`
	RawArgs  []string  `yaml:"args"`
	Groups   []Group   `yaml:"groups"`
	Commands []Command `yaml:"commands"`

	// Indexes
	commandsMap map[string]Command
	groupsMap   map[string]Group
}

type Group struct {
	Shell    string `yaml:"shell,omitempty"`
	Name     string
	Desc     string   `yaml:"desc"`
	Vars     []Var    `yaml:"vars"`
	Args     []Arg    `yaml:"-"`
	RawArgs  []string `yaml:"args"`
	Commands []Command
	Groups   []Group

	// Indexes
	commandsMap map[string]Command
	groupsMap   map[string]Group
}

type Command struct {
	Shell   string `yaml:"shell,omitempty"`
	Name    string
	Desc    string   `yaml:"desc"`
	Vars    []Var    `yaml:"vars"`
	Args    []Arg    `yaml:"-"`
	RawArgs []string `yaml:"args"`
	Script  string   `yaml:"script"`
	Before  string   `yaml:"before"`
	After   string   `yaml:"after"`
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
	Format   string `yaml:"-"` // file format: env, json, toml, etc.
	IsFile   bool   `yaml:"-"` // whether this variable is loaded from a file
}

type SearchResult struct {
	Exists  bool
	Command *Command // non-nil if result is a command
	Groups  []*Group // chain of groups from outermost to innermost
}

func (c *AppConfig) BuildIndexes() error {
	c.commandsMap = make(map[string]Command)
	for _, cmd := range c.Commands {
		if cmd.Name == "" {
			return fmt.Errorf("command with empty name")
		}
		if _, exists := c.commandsMap[cmd.Name]; exists {
			return fmt.Errorf("duplicate command name: %s", cmd.Name)
		}
		c.commandsMap[cmd.Name] = cmd
	}

	c.groupsMap = make(map[string]Group)
	for i, group := range c.Groups {
		if group.Name == "" {
			return fmt.Errorf("group with empty name")
		}
		if _, exists := c.groupsMap[group.Name]; exists {
			return fmt.Errorf("duplicate group name: %s", group.Name)
		}
		if err := c.Groups[i].BuildIndexes(); err != nil {
			return fmt.Errorf("group %s: %w", group.Name, err)
		}
		c.groupsMap[group.Name] = c.Groups[i]
	}
	return nil
}

func (g *Group) BuildIndexes() error {
	g.commandsMap = make(map[string]Command)
	for _, cmd := range g.Commands {
		if cmd.Name == "" {
			return fmt.Errorf("command with empty name")
		}
		if _, exists := g.commandsMap[cmd.Name]; exists {
			return fmt.Errorf("duplicate command name: %s", cmd.Name)
		}
		g.commandsMap[cmd.Name] = cmd
	}

	g.groupsMap = make(map[string]Group)
	for i, sub := range g.Groups {
		if sub.Name == "" {
			return fmt.Errorf("group with empty name")
		}
		if _, exists := g.groupsMap[sub.Name]; exists {
			return fmt.Errorf("duplicate group name: %s", sub.Name)
		}
		if err := g.Groups[i].BuildIndexes(); err != nil {
			return fmt.Errorf("group %s: %w", sub.Name, err)
		}
		g.groupsMap[sub.Name] = g.Groups[i]
	}
	return nil
}

func (c *AppConfig) Find(name string) SearchResult {
	if cmd, found := c.commandsMap[name]; found {
		return SearchResult{Exists: true, Command: &cmd}
	}

	if group, found := c.groupsMap[name]; found {
		return SearchResult{Exists: true, Groups: []*Group{&group}}
	}

	return SearchResult{Exists: false}
}

func (g *Group) Find(name string) SearchResult {
	if cmd, found := g.commandsMap[name]; found {
		return SearchResult{Exists: true, Command: &cmd}
	}

	if sub, found := g.groupsMap[name]; found {
		return SearchResult{Exists: true, Groups: []*Group{&sub}}
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
