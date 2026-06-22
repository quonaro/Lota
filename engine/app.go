package engine

import (
	"context"
	"os"

	"github.com/quonaro/lota/config"
)

// App bundles config, options, and native handlers for a concise embedded API.
type App struct {
	cfg  *config.AppConfig
	opts Options
	name string
}

// NewApp parses embedded YAML data and returns a ready-to-use App.
func NewApp(data []byte, opts Options) (*App, error) {
	cfg, err := LoadConfig(data)
	if err != nil {
		return nil, err
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return &App{cfg: cfg, opts: opts}, nil
}

// NewAppFromPath loads a config from a file path and returns a ready-to-use App.
func NewAppFromPath(path string, opts Options) (*App, error) {
	cfg, configDir, err := LoadConfigFromPath(path)
	if err != nil {
		return nil, err
	}
	if opts.ConfigDir == "" {
		opts.ConfigDir = configDir
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	return &App{cfg: cfg, opts: opts}, nil
}

// Config returns the underlying parsed configuration.
func (a *App) Config() *config.AppConfig {
	return a.cfg
}

// Run resolves args against the config and executes the target command.
func (a *App) Run(ctx context.Context, args []string) error {
	return Run(ctx, a.cfg, args, a.opts)
}

// PrintHelp writes top-level help to the configured stdout using the app name.
func (a *App) PrintHelp() {
	PrintHelp(a.cfg, a.opts.Stdout, a.name)
}

// PrintGroupHelp writes group-specific help to the configured stdout.
func (a *App) PrintGroupHelp(groups []*config.Group) {
	PrintGroupHelp(a.cfg, groups, a.opts.Stdout, a.name)
}

// AppBuilder provides a fluent API for constructing an App.
type AppBuilder struct {
	data    []byte
	path    string
	name    string
	natives map[string]NativeFunc
	opts    Options
}

// NewBuilder creates a builder from embedded YAML data.
func NewBuilder(data []byte) *AppBuilder {
	return &AppBuilder{data: data, natives: make(map[string]NativeFunc)}
}

// NewBuilderFromPath creates a builder from a file path.
func NewBuilderFromPath(path string) *AppBuilder {
	return &AppBuilder{path: path, natives: make(map[string]NativeFunc)}
}

// WithName sets the application name used in help output.
func (b *AppBuilder) WithName(name string) *AppBuilder {
	b.name = name
	return b
}

// WithNative registers a native handler for a command name.
func (b *AppBuilder) WithNative(name string, fn NativeFunc) *AppBuilder {
	b.natives[name] = fn
	return b
}

// WithOptions sets execution options.
func (b *AppBuilder) WithOptions(opts Options) *AppBuilder {
	b.opts = opts
	return b
}

// Build parses the config, registers natives, and returns the App.
func (b *AppBuilder) Build() (*App, error) {
	var app *App
	var err error
	if b.path != "" {
		app, err = NewAppFromPath(b.path, b.opts)
	} else {
		app, err = NewApp(b.data, b.opts)
	}
	if err != nil {
		return nil, err
	}
	app.name = b.name
	for name, fn := range b.natives {
		RegisterNative(name, fn)
	}
	return app, nil
}
