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

// PrintHelp writes top-level help to the configured stdout.
func (a *App) PrintHelp(appName string) {
	PrintHelp(a.cfg, a.opts.Stdout, appName)
}

// PrintGroupHelp writes group-specific help to the configured stdout.
func (a *App) PrintGroupHelp(groups []*config.Group, appName string) {
	PrintGroupHelp(a.cfg, groups, a.opts.Stdout, appName)
}
