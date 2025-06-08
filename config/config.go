package config

import (
	"encoding/json"
	"fmt"
	"os"
	"zwfm-metadata/utils"
)

// Config represents the main configuration
type Config struct {
	WebServerPort int            `json:"webServerPort"`
	Debug         bool           `json:"debug,omitempty"`
	Inputs        []InputConfig  `json:"inputs"`
	Outputs       []OutputConfig `json:"outputs"`
	Formatters    []string       `json:"formatters,omitempty"`
}

// InputConfig represents input configuration
type InputConfig struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Prefix   string                 `json:"prefix,omitempty"`
	Suffix   string                 `json:"suffix,omitempty"`
	Settings map[string]interface{} `json:"settings"`
}

// OutputConfig represents output configuration
type OutputConfig struct {
	Type       string                 `json:"type"`
	Name       string                 `json:"name"`
	Inputs     []string               `json:"inputs"`
	Formatters []string               `json:"formatters,omitempty"`
	Settings   map[string]interface{} `json:"settings"`
}

// DynamicInputSettings represents settings for dynamic input
type DynamicInputSettings struct {
	Secret     string `json:"secret"`
	Expiration struct {
		Type    string `json:"type"` // "dynamic", "fixed", "none"
		Minutes int    `json:"minutes,omitempty"`
	} `json:"expiration"`
}

// URLInputSettings represents settings for URL input
type URLInputSettings struct {
	URL             string `json:"url"`
	JSONParsing     bool   `json:"jsonParsing"`
	JSONKey         string `json:"jsonKey,omitempty"`
	PollingInterval int    `json:"pollingInterval"`
}

// TextInputSettings represents settings for text input
type TextInputSettings struct {
	Text string `json:"text"`
}

// IcecastOutputSettings represents settings for Icecast output
type IcecastOutputSettings struct {
	Delay      int    `json:"delay"`
	Server     string `json:"server"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Mountpoint string `json:"mountpoint"`
}

// FileOutputSettings represents settings for file output
type FileOutputSettings struct {
	Delay    int    `json:"delay"`
	Filename string `json:"filename"`
}

// PostOutputSettings represents settings for POST output with full metadata
type PostOutputSettings struct {
	Delay          int                    `json:"delay"`
	URL            string                 `json:"url"`
	BearerToken    string                 `json:"bearerToken,omitempty"`
	PayloadMapping map[string]interface{} `json:"payloadMapping,omitempty"`
	// TODO: Remove PayloadMappingOmitEmpty when padenc-api properly handles empty fields
	// This is a temporary workaround to exclude empty fields from the payload
	// Once padenc-api can handle empty string values correctly, this field and all
	// related logic in outputs/post.go should be removed
	PayloadMappingOmitEmpty bool `json:"payloadMappingOmitEmpty,omitempty"`
}

// LoadConfig loads configuration from a file
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer utils.CloseFile(file)

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Set default port if not specified
	if config.WebServerPort == 0 {
		config.WebServerPort = 9000
	}

	return &config, nil
}
