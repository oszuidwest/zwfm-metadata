package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

// Config represents the main configuration
type Config struct {
	WebServerPort int            `json:"webServerPort"`
	Debug         bool           `json:"debug,omitempty"`
	StationName   string         `json:"stationName,omitempty"`
	BrandColor    string         `json:"brandColor,omitempty"`
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

// DynamicInputConfig represents configuration for dynamic input
type DynamicInputConfig struct {
	Secret     string `json:"secret"`
	Expiration struct {
		Type    string `json:"type"` // "dynamic", "fixed", "none"
		Minutes int    `json:"minutes,omitempty"`
	} `json:"expiration"`
}

// URLInputConfig represents configuration for URL input
type URLInputConfig struct {
	URL             string `json:"url"`
	JSONParsing     bool   `json:"jsonParsing"`
	JSONKey         string `json:"jsonKey,omitempty"`
	PollingInterval int    `json:"pollingInterval"`
}

// TextInputConfig represents configuration for text input
type TextInputConfig struct {
	Text string `json:"text"`
}

// IcecastOutputConfig represents configuration for Icecast output
type IcecastOutputConfig struct {
	Delay      int    `json:"delay"`
	Server     string `json:"server"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Mountpoint string `json:"mountpoint"`
}

// FileOutputConfig represents configuration for file output
type FileOutputConfig struct {
	Delay    int    `json:"delay"`
	Filename string `json:"filename"`
}

// PostOutputConfig represents configuration for POST output with full metadata
type PostOutputConfig struct {
	Delay          int                    `json:"delay"`
	URL            string                 `json:"url"`
	BearerToken    string                 `json:"bearerToken,omitempty"`
	PayloadMapping map[string]interface{} `json:"payloadMapping,omitempty"`
	// TODO: Remove OmitEmpty when padenc-api properly handles empty fields
	// This is a temporary workaround to exclude empty fields from the payload
	// Once padenc-api can handle empty string values correctly, this field and all
	// related logic in outputs/post.go should be removed
	OmitEmpty bool `json:"omitEmpty,omitempty"`
}

// DLSPlusOutputConfig represents configuration for DLS Plus output
type DLSPlusOutputConfig struct {
	Delay    int    `json:"delay"`
	Filename string `json:"filename"`
}

// WebSocketOutputConfig represents configuration for WebSocket output
type WebSocketOutputConfig struct {
	Delay          int                    `json:"delay"`
	Path           string                 `json:"path"`
	PayloadMapping map[string]interface{} `json:"payloadMapping,omitempty"`
}

// HTTPOutputConfig represents configuration for HTTP output
type HTTPOutputConfig struct {
	Delay     int            `json:"delay"`
	Endpoints []HTTPEndpoint `json:"endpoints"`
}

// HTTPEndpoint represents a single HTTP endpoint configuration
type HTTPEndpoint struct {
	Path           string                 `json:"path"`
	ResponseType   string                 `json:"responseType,omitempty"` // json, xml, plaintext, yaml, custom
	PayloadMapping map[string]interface{} `json:"payloadMapping,omitempty"`
}

// LoadConfig loads configuration from a file
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			slog.Warn("Failed to close config file", "error", err)
		}
	}()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Set default port if not specified
	if config.WebServerPort == 0 {
		config.WebServerPort = 9000
	}

	// Set default station name if not specified
	if config.StationName == "" {
		config.StationName = "ZuidWest FM"
	}

	// Set default brand color if not specified
	if config.BrandColor == "" {
		config.BrandColor = "#e6007e"
	}

	return &config, nil
}
