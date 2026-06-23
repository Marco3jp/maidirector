package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig       `yaml:"server"`
	Store       StoreConfig        `yaml:"store"`
	GitHub      GitHubConfig       `yaml:"github"`
	LayerChains []LayerChainConfig `yaml:"layer_chains"`
}

type GitHubConfig struct {
	Org            string   `yaml:"org"`
	ExcludeAuthors []string `yaml:"exclude_authors"`
	BotPolicy      string   `yaml:"bot_policy"` // "exclude" (default) or priority number: 1 (highest) - 5 (lowest)
}

type LayerChainConfig struct {
	Name   string   `yaml:"name" json:"name"`
	Layers []string `yaml:"layers" json:"layers"`
}

type ServerConfig struct {
	Port int `yaml:"port"`
}

type StoreConfig struct {
	SQLitePath string `yaml:"sqlite_path"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{Port: 20135},
		Store: StoreConfig{
			SQLitePath: "~/.maidirector/data.db",
		},
		LayerChains: []LayerChainConfig{
			{Name: "default", Layers: []string{"proto", "microservice", "gateway", "frontend"}},
			{Name: "simple", Layers: []string{"backend", "frontend"}},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) ResolveSQLitePath() string {
	p := c.Store.SQLitePath
	if len(p) > 0 && p[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			p = home + p[1:]
		}
	}
	return p
}
