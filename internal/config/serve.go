package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ServeConfig is machine-local; it never travels with wiki content.
type ServeConfig struct {
	Addr          string   `toml:"addr"`
	AuthTokenFile string   `toml:"auth_token_file"`
	PublicOrigin  string   `toml:"public_origin"`
	AllowHosts    []string `toml:"allow_hosts"`
}

func LoadServeConfig() (ServeConfig, error) {
	var global struct {
		Serve ServeConfig `toml:"serve"`
	}
	global.Serve.Addr = "localhost:8737"
	md, err := toml.DecodeFile(filepath.Join(ConfigHome(), "config.toml"), &global)
	if os.IsNotExist(err) {
		return global.Serve, nil
	}
	if err != nil {
		return ServeConfig{}, fmt.Errorf("invalid global configuration (check config.toml syntax and types)")
	}
	for _, key := range md.Undecoded() {
		if len(key) > 0 && key[0] == "serve" {
			return ServeConfig{}, fmt.Errorf("unknown serve configuration field")
		}
	}
	return global.Serve, nil
}
