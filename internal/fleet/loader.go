package fleet

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

var envVarRegex = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Config represents the shield configuration file.
type Config struct {
	Tenants []TenantConfig `yaml:"tenants"`
	Tokens  []TokenConfig  `yaml:"tokens"`
}

// TenantConfig represents a tenant in the config file.
type TenantConfig struct {
	ID     string        `yaml:"id"`
	Mode   string        `yaml:"mode"` // "isolated" or "fleet"
	Agents []AgentConfig `yaml:"agents"`
}

// AgentConfig represents an agent in the config file.
type AgentConfig struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	IP          string `yaml:"ip"`
	WebhookURL  string `yaml:"webhook_url"`
	Tier        string `yaml:"tier"` // "commodore", "captain", "crew"
	Description string `yaml:"description"`
}

// TokenConfig represents an auth token in the config file.
type TokenConfig struct {
	Token    string `yaml:"token"`
	TenantID string `yaml:"tenant_id"`
	Name     string `yaml:"name"` // Optional human-readable name
}

// LoadConfig loads fleet configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables
	content := envVarRegex.ReplaceAllStringFunc(string(data), func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := match[2 : len(match)-1]
		if value := os.Getenv(varName); value != "" {
			return value
		}
		return match // Keep original if not set
	})

	var config Config
	if err := yaml.Unmarshal([]byte(content), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// ApplyConfig applies a config to a fleet manager.
func ApplyConfig(mgr *Manager, config *Config) {
	for _, tc := range config.Tenants {
		// Create tenant
		mgr.CreateTenant(tc.ID)

		// Set mode
		if tc.Mode == "fleet" {
			mgr.SetMode(tc.ID, Fleet)
		} else {
			mgr.SetMode(tc.ID, Isolated)
		}

		// Add agents
		for _, ac := range tc.Agents {
			tier := ac.Tier
			if tier == "" {
				tier = "crew" // Default tier
			}
			agent := Agent{
				ID:          ac.ID,
				Name:        ac.Name,
				IP:          ac.IP,
				WebhookURL:  ac.WebhookURL,
				Tier:        tier,
				Description: ac.Description,
			}
			mgr.AddAgent(tc.ID, agent)
		}
	}
}

// LoadAndApply loads config from file and applies to manager.
func LoadAndApply(mgr *Manager, path string) error {
	config, err := LoadConfig(path)
	if err != nil {
		return err
	}
	ApplyConfig(mgr, config)
	return nil
}
