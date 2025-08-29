package utils

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Patterns  map[string]PatternConfig `yaml:"patterns"`
	Crawler   CrawlerConfig            `yaml:"crawler"`
	Scanner   ScannerConfig            `yaml:"scanner"`
	Discovery DiscoveryConfig          `yaml:"discovery"`
	Wordlists WordlistsConfig          `yaml:"wordlists"`
}

// PatternConfig represents a regex pattern configuration
type PatternConfig struct {
	Pattern     string `yaml:"pattern"`
	Description string `yaml:"description"`
	Confidence  string `yaml:"confidence"`
	Enabled     bool   `yaml:"enabled,omitempty"`
}

// CrawlerConfig represents crawler settings
type CrawlerConfig struct {
	MaxDepth     int    `yaml:"max_depth"`
	Threads      int    `yaml:"threads"`
	Timeout      int    `yaml:"timeout"`
	UserAgent    string `yaml:"user_agent"`
	IgnoreRobots bool   `yaml:"ignore_robots"`
}

// ScannerConfig represents scanner settings
type ScannerConfig struct {
	Threads      int    `yaml:"threads"`
	Timeout      int    `yaml:"timeout"`
	OutputFormat string `yaml:"output_format"`
}

// DiscoveryConfig represents discovery settings
type DiscoveryConfig struct {
	Threads      int    `yaml:"threads"`
	Timeout      int    `yaml:"timeout"`
	MaxRedirects int    `yaml:"max_redirects"`
	StatusFilter string `yaml:"status_filter"`
	UserAgent    string `yaml:"user_agent"`
}

// WordlistsConfig represents wordlist configurations
type WordlistsConfig struct {
	CommonEndpoints []string `yaml:"common_endpoints"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		// Try default locations
		defaultPaths := []string{
			"./config.yaml",
			"./config/config.yaml",
			"./config/patterns.yaml",
			"~/.jsfinder/config.yaml",
		}
		
		for _, path := range defaultPaths {
			if _, err := os.Stat(path); err == nil {
				configPath = path
				break
			}
		}
		
		if configPath == "" {
			// Return default configuration
			return getDefaultConfig(), nil
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Merge with defaults for missing values
	defaultConfig := getDefaultConfig()
	mergeConfigs(&config, defaultConfig)

	return &config, nil
}

// GetCompiledPatterns returns compiled regex patterns from config
func (c *Config) GetCompiledPatterns() (map[string]*regexp.Regexp, error) {
	patterns := make(map[string]*regexp.Regexp)
	
	for name, patternConfig := range c.Patterns {
		// Skip disabled patterns
		if patternConfig.Enabled == false {
			continue
		}
		
		compiled, err := regexp.Compile(patternConfig.Pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern '%s': %w", name, err)
		}
		
		patterns[name] = compiled
	}
	
	return patterns, nil
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(config *Config, configPath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

func getDefaultConfig() *Config {
	return &Config{
		Patterns: getDefaultPatterns(),
		Crawler: CrawlerConfig{
			MaxDepth:     3,
			Threads:      10,
			Timeout:      30,
			UserAgent:    "jsfinder/1.0",
			IgnoreRobots: false,
		},
		Scanner: ScannerConfig{
			Threads:      10,
			Timeout:      30,
			OutputFormat: "json",
		},
		Discovery: DiscoveryConfig{
			Threads:      20,
			Timeout:      10,
			MaxRedirects: 3,
			StatusFilter: "200,201,202,204,301,302,307,308,401,403",
			UserAgent:    "jsfinder/1.0",
		},
		Wordlists: WordlistsConfig{
			CommonEndpoints: []string{
				"api", "admin", "login", "auth", "users", "user",
				"config", "settings", "dashboard", "profile",
				"account", "accounts", "data", "info", "status",
				"health", "version", "v1", "v2", "v3",
			},
		},
	}
}

func getDefaultPatterns() map[string]PatternConfig {
	return map[string]PatternConfig{
		"AWS_ACCESS_KEY": {
			Pattern:     `(?i)(aws_access_key_id|aws_access_key|aws_key_id)[\s]*[:=][\s]*["']?([A-Z0-9]{20})["']?`,
			Description: "AWS Access Key ID",
			Confidence:  "HIGH",
			Enabled:     true,
		},
		"AWS_SECRET_KEY": {
			Pattern:     `(?i)(aws_secret_access_key|aws_secret_key)[\s]*[:=][\s]*["']?([A-Za-z0-9/+=]{40})["']?`,
			Description: "AWS Secret Access Key",
			Confidence:  "HIGH",
			Enabled:     true,
		},
		"JWT_TOKEN": {
			Pattern:     `(?i)(jwt|token)[\s]*[:=][\s]*["']?(eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*)["']?`,
			Description: "JSON Web Token",
			Confidence:  "HIGH",
			Enabled:     true,
		},
		"API_KEY": {
			Pattern:     `(?i)(api_key|apikey|api-key)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{16,})["']?`,
			Description: "Generic API Key",
			Confidence:  "MEDIUM",
			Enabled:     true,
		},
		"SECRET": {
			Pattern:     `(?i)(secret|secret_key)[\s]*[:=][\s]*["']?([A-Za-z0-9_-]{16,})["']?`,
			Description: "Secret Key",
			Confidence:  "MEDIUM",
			Enabled:     true,
		},
		"API_ENDPOINT": {
			Pattern:     `(?i)["'](https?://[^"'\s]*/(api|admin|v[0-9]+)/[^"'\s]*)["']`,
			Description: "API Endpoint URL",
			Confidence:  "LOW",
			Enabled:     true,
		},
	}
}

func mergeConfigs(target, source *Config) {
	// Merge patterns
	for name, pattern := range source.Patterns {
		if _, exists := target.Patterns[name]; !exists {
			if target.Patterns == nil {
				target.Patterns = make(map[string]PatternConfig)
			}
			target.Patterns[name] = pattern
		}
	}

	// Merge crawler config
	if target.Crawler.MaxDepth == 0 {
		target.Crawler.MaxDepth = source.Crawler.MaxDepth
	}
	if target.Crawler.Threads == 0 {
		target.Crawler.Threads = source.Crawler.Threads
	}
	if target.Crawler.Timeout == 0 {
		target.Crawler.Timeout = source.Crawler.Timeout
	}
	if target.Crawler.UserAgent == "" {
		target.Crawler.UserAgent = source.Crawler.UserAgent
	}

	// Merge scanner config
	if target.Scanner.Threads == 0 {
		target.Scanner.Threads = source.Scanner.Threads
	}
	if target.Scanner.Timeout == 0 {
		target.Scanner.Timeout = source.Scanner.Timeout
	}
	if target.Scanner.OutputFormat == "" {
		target.Scanner.OutputFormat = source.Scanner.OutputFormat
	}

	// Merge discovery config
	if target.Discovery.Threads == 0 {
		target.Discovery.Threads = source.Discovery.Threads
	}
	if target.Discovery.Timeout == 0 {
		target.Discovery.Timeout = source.Discovery.Timeout
	}
	if target.Discovery.MaxRedirects == 0 {
		target.Discovery.MaxRedirects = source.Discovery.MaxRedirects
	}
	if target.Discovery.StatusFilter == "" {
		target.Discovery.StatusFilter = source.Discovery.StatusFilter
	}
	if target.Discovery.UserAgent == "" {
		target.Discovery.UserAgent = source.Discovery.UserAgent
	}
}