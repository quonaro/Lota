package engine

import (
	"context"
	"os"

	"github.com/quonaro/lota/config"
)

// App bundles config, options, and native handlers for a concise embedded API.
type App struct {
	cfg     *config.AppConfig
	opts    Options
	name    string
	natives map[string]NativeFunc
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
	return &App{cfg: cfg, opts: opts, natives: make(map[string]NativeFunc)}, nil
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
	return &App{cfg: cfg, opts: opts, natives: make(map[string]NativeFunc)}, nil
}

// Config returns the underlying parsed configuration.
func (a *App) Config() *config.AppConfig {
	return a.cfg
}

// Run resolves args against the config and executes the target command.
func (a *App) Run(ctx context.Context, args []string) error {
	opts := a.opts
	if len(a.natives) > 0 {
		if opts.NativeHandlers == nil {
			opts.NativeHandlers = a.natives
		} else {
			merged := make(map[string]NativeFunc, len(opts.NativeHandlers)+len(a.natives))
			for k, v := range opts.NativeHandlers {
				merged[k] = v
			}
			for k, v := range a.natives {
				merged[k] = v
			}
			opts.NativeHandlers = merged
		}
	}
	return Run(ctx, a.cfg, args, opts)
}

// PrintHelp writes top-level help to the configured stdout using the app name.
func (a *App) PrintHelp() {
	PrintHelp(a.cfg, a.opts.Stdout, a.name)
}

// PrintGroupHelp writes group-specific help to the configured stdout.
func (a *App) PrintGroupHelp(groups []*config.Group) {
	PrintGroupHelp(a.cfg, groups, a.opts.Stdout, a.name)
}

// AppBuilder provides a class-like API for constructing an App.
// Create it with NewBuilder, register natives with RegisterNative, then Build.
type AppBuilder struct {
	name    string
	data    []byte
	path    string
	natives map[string]NativeFunc
	opts    Options
}

// NewBuilder creates a builder from embedded YAML data.
func NewBuilder(name string, data []byte) *AppBuilder {
	return &AppBuilder{name: name, data: data, natives: make(map[string]NativeFunc)}
}

// NewBuilderFromPath creates a builder from a file path.
func NewBuilderFromPath(name, path string) *AppBuilder {
	return &AppBuilder{name: name, path: path, natives: make(map[string]NativeFunc)}
}

// RegisterNative registers a native handler for a command path.
// Root commands use the command name; nested commands use the dot-separated path.
func (b *AppBuilder) RegisterNative(name string, fn NativeFunc) *AppBuilder {
	b.natives[name] = fn
	return b
}

// WithOptions sets execution options.
func (b *AppBuilder) WithOptions(opts Options) *AppBuilder {
	b.opts = opts
	return b
}

// Build parses the config, attaches native handlers, and returns the App.
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
	app.natives = b.natives
	return app, nil
}
