// Package rules provides the rule engine for filtering traffic.
package rules

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadFromFile reads a YAML rules file and returns a RuleSet.
func LoadFromFile(path string) (*RuleSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	return LoadFromBytes(data)
}

// LoadFromBytes parses YAML bytes into a RuleSet.
func LoadFromBytes(data []byte) (*RuleSet, error) {
	var rs RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	return &rs, nil
}

// SaveToFile writes a RuleSet to a YAML file.
func SaveToFile(rs *RuleSet, path string) error {
	data, err := yaml.Marshal(rs)
	if err != nil {
		return fmt.Errorf("failed to marshal rules: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write rules file: %w", err)
	}

	return nil
}
