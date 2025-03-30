package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config stores application configuration
type Config struct {
	DefaultProvider string      `yaml:"defaultProvider"`
	OpenAI          OpenAIConfig `yaml:"openai"`
	CBOE            CBOEConfig   `yaml:"cboe"`
	Gemini          GeminiConfig `yaml:"gemini"`
}

// OpenAIConfig stores OpenAI-specific configuration
type OpenAIConfig struct {
	APIKey string `yaml:"apiKey"`
	Model  string `yaml:"model"`
}

// CBOEConfig stores CBOE-specific configuration
type CBOEConfig struct {
	Email      string `yaml:"email"`      // Email for CBOE authentication
	Token      string `yaml:"token"`      // Token for CBOE authentication
	Endpoint   string `yaml:"endpoint"`   // API endpoint
	Model      string `yaml:"model"`      // Model to use
	Datasource string `yaml:"datasource"` // Default datasource to use if any
}

// GeminiConfig stores Google Gemini-specific configuration
type GeminiConfig struct {
	APIKey string `yaml:"apiKey"` // API key for Gemini authentication
	Model  string `yaml:"model"`  // Model to use
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".llm-tool.yaml" // Fallback to current directory
	}
	
	configDir := filepath.Join(homeDir, ".config", "llm-tool")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
	}
	
	return filepath.Join(configDir, "config.yaml")
}

// Load loads configuration from file
func Load() (*Config, error) {
	configPath := GetConfigPath()
	
	// Default config
	config := &Config{
		DefaultProvider: "openai",
		OpenAI: OpenAIConfig{
			Model: "gpt-4o-mini",
		},
		CBOE: CBOEConfig{
			Endpoint: "https://api.cboe.com/llm/v1",
			Model:    "default",
		},
		Gemini: GeminiConfig{
			Model: "gemini-2.0-flash-lite",
		},
	}
	
	// Check if config file exists
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}
	
	return config, nil
}

// Save saves the configuration to a file
func (c *Config) Save() error {
	configPath := GetConfigPath()
	
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	err = os.WriteFile(configPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	return nil
}
