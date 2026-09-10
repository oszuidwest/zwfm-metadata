// Package config provides configuration management for the metadata router
// including loading and validation of input, output, and formatter settings.
package config

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
)

// Config holds application settings including web server port, inputs, and outputs.
type Config struct {
	WebServerPort int            `json:"webServerPort"`
	Debug         bool           `json:"debug,omitempty"`
	StationName   string         `json:"stationName,omitempty"`
	BrandColor    string         `json:"brandColor,omitempty"`
	Inputs        []InputConfig  `json:"inputs"`
	Outputs       []OutputConfig `json:"outputs"`
}

// InputConfig defines a metadata source with its type, name, and type-specific settings.
type InputConfig struct {
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Prefix   string          `json:"prefix,omitempty"`
	Suffix   string          `json:"suffix,omitempty"`
	Filters  []FilterConfig  `json:"filters,omitempty"`
	Settings json.RawMessage `json:"settings"`
}

// OutputConfig defines a metadata destination with its type, linked inputs, and type-specific settings.
type OutputConfig struct {
	Type       string          `json:"type"`
	Name       string          `json:"name"`
	Inputs     []string        `json:"inputs"`
	Formatters []string        `json:"formatters,omitempty"`
	Settings   json.RawMessage `json:"settings"`
}

// FilterConfig defines a metadata filter with a type and type-specific settings.
type FilterConfig struct {
	Type       string `json:"type"`
	Field      string `json:"field,omitempty"`      // For pattern filter
	Pattern    string `json:"pattern,omitempty"`    // For pattern filter
	Action     string `json:"action,omitempty"`     // For pattern filter
	MinSeconds int    `json:"minSeconds,omitempty"` // For duration filter
}

// DynamicInputConfig holds settings for HTTP API-driven metadata updates with optional expiration.
type DynamicInputConfig struct {
	Secret     string `json:"secret"`
	Expiration struct {
		Type    string `json:"type"`              // "dynamic", "fixed", "none"
		Minutes int    `json:"minutes,omitempty"` // Fallback minutes for dynamic, or fixed duration
	} `json:"expiration"`
}

// URLInputConfig holds settings for polling external URLs with optional JSON parsing.
type URLInputConfig struct {
	URL             string `json:"url"`
	JSONParsing     bool   `json:"jsonParsing"`
	JSONKey         string `json:"jsonKey,omitempty"`
	ExpiryKey       string `json:"expiryKey,omitempty"`
	ExpiryFormat    string `json:"expiryFormat,omitempty"`
	PollingInterval int    `json:"pollingInterval"`
}

// TextInputConfig holds settings for static text metadata, typically used as fallback.
type TextInputConfig struct {
	Text string `json:"text"`
}

// IcecastOutputConfig holds connection settings for updating Icecast stream metadata.
type IcecastOutputConfig struct {
	Server     string `json:"server"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Mountpoint string `json:"mountpoint"`
}

// FileOutputConfig holds settings for writing metadata to local files.
type FileOutputConfig struct {
	Filename string `json:"filename"`
}

// URLOutputConfig holds settings for sending metadata via HTTP GET or POST requests.
type URLOutputConfig struct {
	URL            string         `json:"url"`
	Method         string         `json:"method,omitempty"` // GET or POST (required)
	BearerToken    string         `json:"bearerToken,omitempty"`
	PayloadMapping map[string]any `json:"payloadMapping,omitempty"` // Only for POST
}

// DLPlusOutputConfig holds settings for DAB/DAB+ DL Plus text output.
type DLPlusOutputConfig struct {
	Filename string `json:"filename"`
}

// WebSocketOutputConfig holds settings for real-time WebSocket metadata broadcasting.
type WebSocketOutputConfig struct {
	Path           string         `json:"path"`
	PayloadMapping map[string]any `json:"payloadMapping,omitempty"`
}

// HTTPOutputConfig holds settings for serving metadata via HTTP GET endpoints.
type HTTPOutputConfig struct {
	Endpoints []HTTPEndpoint `json:"endpoints"`
}

// HTTPEndpoint defines a single HTTP GET endpoint with response format and optional payload mapping.
type HTTPEndpoint struct {
	Path           string         `json:"path"`
	ResponseType   string         `json:"responseType,omitempty"` // json, xml, plaintext
	PayloadMapping map[string]any `json:"payloadMapping,omitempty"`
}

// StereoToolOutputConfig holds connection settings for Stereo Tool metadata updates.
type StereoToolOutputConfig struct {
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
}

// LoadConfig reads a configuration from the specified file.
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename) // #nosec G304 -- intentionally loads the user-specified config file
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	config.WebServerPort = cmp.Or(config.WebServerPort, 9000)
	config.StationName = cmp.Or(config.StationName, "ZuidWest FM")
	config.BrandColor = cmp.Or(config.BrandColor, "#e6007e")

	return &config, nil
}
