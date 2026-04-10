package config

import "fmt"

type AppConfig struct {
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
	Name     string
	Desc     string    `yaml:"desc"`
	Vars     []Var     `yaml:"vars"`
	Args     []Arg     `yaml:"-"`
	RawArgs  []string  `yaml:"args"`
	Groups   []Group   `yaml:"groups"`
	Commands []Command
}

type Command struct {
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
	Name     string `yaml:"name"`
	Short    string `yaml:"short"`
	Type     string `yaml:"type"`
	Default  string `yaml:"default"`
	Required bool   `yaml:"required"`
	Wildcard bool   `yaml:"wildcard"`
	MaxArr   *int   `yaml:"max_arr"`
}

type Var struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type SearchResult struct {
	Exists  bool
	Command *Command // non-nil if result is a command
	Group   *Group   // non-nil if result is a group
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
	for _, group := range c.Groups {
		if group.Name == "" {
			return fmt.Errorf("group with empty name")
		}
		if _, exists := c.groupsMap[group.Name]; exists {
			return fmt.Errorf("duplicate group name: %s", group.Name)
		}
		c.groupsMap[group.Name] = group
	}
	// TODO: handle nested groups recursively
	return nil
}

func (c *AppConfig) Find(name string) SearchResult {
	if cmd, found := c.commandsMap[name]; found {
		return SearchResult{Exists: true, Command: &cmd}
	}

	if group, found := c.groupsMap[name]; found {
		return SearchResult{Exists: true, Group: &group}
	}

	return SearchResult{Exists: false}
}
