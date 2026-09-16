package pkg

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// AuthCredentials holds iKuai login for one auth_module.
type AuthCredentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// ExporterConfig is loaded from --config.file for multi-target /probe mode.
type ExporterConfig struct {
	DefaultAuth            string            `yaml:"default_auth"`
	Timeout                int               `yaml:"timeout"`
	InsecureSkipTLS        bool              `yaml:"insecure_skip_tls"`
	Modules                []string          `yaml:"modules"`
	SessionDetail          bool              `yaml:"session_detail"`
	SessionDetailLimit     int               `yaml:"session_detail_limit"`
	DNATSessionDetail      bool              `yaml:"dnat_session_detail"`
	DNATSessionDetailLimit int               `yaml:"dnat_session_detail_limit"`
	AllowedCIDRs           []string          `yaml:"allowed_cidrs"`
	Auths                  map[string]AuthCredentials `yaml:"auths"`
}

// LoadExporterConfig reads and validates exporter YAML config.
func LoadExporterConfig(path string) (*ExporterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &ExporterConfig{
		DefaultAuth:            "default",
		Timeout:                2,
		InsecureSkipTLS:        true,
		SessionDetailLimit:     200,
		DNATSessionDetailLimit: 500,
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.DefaultAuth == "" {
		cfg.DefaultAuth = "default"
	}
	if len(cfg.Auths) == 0 {
		return nil, fmt.Errorf("config: auths must not be empty")
	}
	if _, ok := cfg.Auths[cfg.DefaultAuth]; !ok {
		return nil, fmt.Errorf("config: default_auth %q not found in auths", cfg.DefaultAuth)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2
	}
	if len(cfg.Modules) == 0 {
		cfg.Modules = []string{"sysStat", "lanDevice", "interfaceInfo", "dnat", "session"}
	}

	return cfg, nil
}

// Auth resolves credentials for the given module name.
func (c *ExporterConfig) Auth(module string) (AuthCredentials, error) {
	name := strings.TrimSpace(module)
	if name == "" {
		name = c.DefaultAuth
	}
	auth, ok := c.Auths[name]
	if !ok {
		return AuthCredentials{}, fmt.Errorf("unknown auth_module %q", name)
	}
	if strings.TrimSpace(auth.Username) == "" {
		return AuthCredentials{}, fmt.Errorf("auth_module %q: username is empty", name)
	}
	return auth, nil
}
