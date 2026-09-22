// Package config loads $XDG_CONFIG_HOME/hudiff/config.json (spec §4). Missing
// file means all defaults; a present file only has to carry what it changes.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Web struct {
	Port    int    `json:"port"`
	Open    bool   `json:"open"`
	Browser string `json:"browser"`
}

type Diagram struct {
	Sketch bool `json:"sketch"`
}

type Hooks struct {
	OnSubmit        string `json:"on_submit"`
	OnReviewRequest string `json:"on_review_request"`
}

type UI struct {
	Theme    string `json:"theme"`
	Language string `json:"language"`
}

type Config struct {
	Viewer     string            `json:"viewer"`
	Web        Web               `json:"web"`
	Diagram    Diagram           `json:"diagram"`
	DataDir    string            `json:"data_dir"`
	PromptsDir string            `json:"prompts_dir"`
	Hooks      Hooks             `json:"hooks"`
	Keymap     map[string]string `json:"keymap"`
	UI         UI                `json:"ui"`
}

func Default() Config {
	return Config{
		Viewer: "web",
		Web:    Web{Open: true},
		Keymap: map[string]string{},
		UI:     UI{Theme: "auto", Language: "en"},
	}
}

func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "hudiff")
}

func Path() string { return filepath.Join(Dir(), "config.json") }

// Load is memoised: the file is read once per process.
var Load = sync.OnceValue(func() Config {
	c := Default()
	b, err := os.ReadFile(Path())
	if err != nil {
		return c
	}
	if json.Unmarshal(b, &c) != nil {
		return Default()
	}
	if c.Keymap == nil {
		c.Keymap = map[string]string{}
	}
	return c
})

// StateDir is where sessions live: data_dir, else the XDG state default.
func StateDir() string {
	if d := os.Getenv("HUDIFF_STATE"); d != "" {
		return d
	}
	if d := Load().DataDir; d != "" {
		return expand(d)
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "hudiff")
}

// PromptsDir holds the preset review requests.
func PromptsDir() string {
	if d := os.Getenv("HUDIFF_PROMPTS"); d != "" {
		return d
	}
	if d := Load().PromptsDir; d != "" {
		return expand(d)
	}
	return filepath.Join(Dir(), "prompts")
}

func expand(p string) string {
	if len(p) > 1 && p[0] == '~' && p[1] == '/' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}
