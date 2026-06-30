package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Game     GameConfig     `yaml:"game"`
	Auth     AuthConfig     `yaml:"auth"`
}

type ServerConfig struct {
	HTTPAddr string `yaml:"http_addr"`
	WSAddr   string `yaml:"ws_addr"`
	Host     string `yaml:"host"`
}

type DatabaseConfig struct {
	Postgres string `yaml:"postgres"`
	Redis    string `yaml:"redis"`
}

type GameConfig struct {
	InitialGold       int64 `yaml:"initial_gold"`
	HeartbeatInterval int   `yaml:"heartbeat_interval"`
	MaxReconnectWait  int   `yaml:"max_reconnect_wait"`
	AIOpponentEnabled bool  `yaml:"ai_opponent_enabled"`
}

type AuthConfig struct {
	HTTPSalt  string `yaml:"http_salt"`
	GuestSalt string `yaml:"guest_salt"`
	JWTSecret string `yaml:"jwt_secret"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
