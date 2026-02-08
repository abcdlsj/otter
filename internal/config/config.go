package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ModelConfig struct {
	Name    string `toml:"name"`
	Alias   string `toml:"alias,omitempty"`
	Default bool   `toml:"default,omitempty"`
}

type ProviderConfig struct {
	Name    string            `toml:"name"`
	BaseURL string            `toml:"base_url"`
	APIKey  string            `toml:"api_key"`
	Headers map[string]string `toml:"headers,omitempty"`
	Models  []ModelConfig     `toml:"models"`
	Default bool              `toml:"default,omitempty"`
}

type Config struct {
	Providers []ProviderConfig `toml:"providers"`
	Stream    bool             `toml:"stream"`
	MaxSteps  int              `toml:"max_steps"`

	// 当前选中的 provider 和 model（运行时）
	currentProviderIdx int
	currentModelIdx    int
}

var C Config

func Load() error {
	C = Config{
		Stream:   false,
		MaxSteps: 100,
	}

	home := Home()
	path := filepath.Join(home, "config.toml")
	if _, err := os.Stat(path); err == nil {
		if _, err := toml.DecodeFile(path, &C); err != nil {
			return err
		}
	}

	// 初始化当前选中的 provider 和 model
	C.initCurrentSelection()

	return nil
}

func (c *Config) initCurrentSelection() {
	for i, p := range c.Providers {
		if p.Default {
			c.currentProviderIdx = i
			c.currentModelIdx = defaultModelIdx(p.Models)
			return
		}
	}
	if len(c.Providers) == 0 {
		return
	}
	c.currentProviderIdx = 0
	c.currentModelIdx = defaultModelIdx(c.Providers[0].Models)
}

func defaultModelIdx(models []ModelConfig) int {
	for i, m := range models {
		if m.Default {
			return i
		}
	}
	return 0
}

func (c *Config) CurrentProvider() *ProviderConfig {
	if c.currentProviderIdx >= len(c.Providers) {
		return nil
	}
	return &c.Providers[c.currentProviderIdx]
}

func (c *Config) CurrentModel() *ModelConfig {
	p := c.CurrentProvider()
	if p == nil || c.currentModelIdx >= len(p.Models) {
		return nil
	}
	return &p.Models[c.currentModelIdx]
}

func (c *Config) CurrentModelName() string {
	m := c.CurrentModel()
	if m == nil {
		return "unknown"
	}
	if m.Alias != "" {
		return m.Alias
	}
	return m.Name
}

func (c *Config) CurrentProviderName() string {
	p := c.CurrentProvider()
	if p == nil {
		return "unknown"
	}
	return p.Name
}

func (c *Config) SetModel(providerName, modelName string) bool {
	for i := range c.Providers {
		p := &c.Providers[i]
		p.Default = false
		for j := range p.Models {
			p.Models[j].Default = false
		}
		if p.Name == providerName {
			for j := range p.Models {
				m := &p.Models[j]
				if m.Name == modelName || m.Alias == modelName {
					c.currentProviderIdx = i
					c.currentModelIdx = j
					p.Default = true
					m.Default = true
					return true
				}
			}
		}
	}
	return false
}

func (c *Config) ListModels() []string {
	var result []string
	for _, p := range c.Providers {
		for _, m := range p.Models {
			display := m.Name
			if m.Alias != "" {
				display = m.Alias
			}
			result = append(result, p.Name+"/"+display)
		}
	}
	return result
}

func Save() error {
	home := Home()
	if err := os.MkdirAll(home, 0755); err != nil {
		return err
	}

	path := filepath.Join(home, "config.toml")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return toml.NewEncoder(f).Encode(C)
}

func Home() string {
	if h := os.Getenv("AGENT_HOME"); h != "" {
		return h
	}
	return filepath.Join(os.Getenv("HOME"), ".config", "otter")
}

func SessionsDir() string {
	return filepath.Join(Home(), "sessions", WorkDirName())
}

func WorkDirName() string {
	wd, _ := os.Getwd()
	if wd == "" {
		return "default"
	}

	last := filepath.Base(wd)
	parent := filepath.Base(filepath.Dir(wd))

	if parent == "." || parent == "/" {
		return last
	}
	return parent + "_" + last
}
