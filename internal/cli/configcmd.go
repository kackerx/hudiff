package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kackerx/hudiff/internal/config"
)

type ConfigPathCmd struct{}

func (c *ConfigPathCmd) Run(g *Globals) error {
	fmt.Println(config.Path())
	return nil
}

type ConfigShowCmd struct{}

// Run prints the effective config: defaults with the file layered on top.
func (c *ConfigShowCmd) Run(g *Globals) error {
	cfg := config.Load()
	cfg.Keymap = cfg.MergedKeymap()
	return printJSON(cfg)
}

type ConfigInitCmd struct {
	Force bool `help:"Overwrite an existing config file."`
}

func (c *ConfigInitCmd) Run(g *Globals) error {
	path := config.Path()
	if _, err := os.Stat(path); err == nil && !c.Force {
		return usage("%s already exists; --force to overwrite", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return envErr("%v", err)
	}
	cfg := config.Default()
	cfg.Keymap = config.Keymap
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return envErr("%v", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return envErr("%v", err)
	}
	fmt.Println("wrote " + path)
	return nil
}
