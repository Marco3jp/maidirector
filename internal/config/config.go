package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server      ServerConfig       `yaml:"server"`
	LLM         LLMConfig          `yaml:"llm"`
	Store       StoreConfig        `yaml:"store"`
	Qdrant      QdrantConfig       `yaml:"qdrant"`
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

type LLMConfig struct {
	Provider       string `yaml:"provider"`
	ChatModel      string `yaml:"chat_model"`
	EmbeddingModel string `yaml:"embedding_model"`
	Endpoint       string `yaml:"endpoint"`
}

type StoreConfig struct {
	SQLitePath string `yaml:"sqlite_path"`
}

type QdrantConfig struct {
	Endpoint   string `yaml:"endpoint"`
	Collection string `yaml:"collection"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{Port: 20135},
		LLM: LLMConfig{
			Provider:       "ollama",
			ChatModel:      "qwen2.5:14b",
			EmbeddingModel: "nomic-embed-text",
			Endpoint:       "http://localhost:11434",
		},
		Store: StoreConfig{
			SQLitePath: "~/.maidirector/data.db",
		},
		Qdrant: QdrantConfig{
			Endpoint:   "http://localhost:6333",
			Collection: "maidirector",
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
