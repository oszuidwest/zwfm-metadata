package utils

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"text/template"
	"time"
)

// bufferPool is a pool of reusable bytes.Buffer objects for template processing
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// PayloadMapper handles custom payload transformation based on configuration
type PayloadMapper struct {
	mapping   map[string]interface{}
	omitEmpty bool
}

// NewPayloadMapper creates a new payload mapper with the given mapping configuration
func NewPayloadMapper(mapping map[string]interface{}) *PayloadMapper {
	return &PayloadMapper{
		mapping: mapping,
	}
}

// WithOmitEmpty creates a new payload mapper that omits empty values
func NewPayloadMapperWithOmitEmpty(mapping map[string]interface{}, omitEmpty bool) *PayloadMapper {
	return &PayloadMapper{
		mapping:   mapping,
		omitEmpty: omitEmpty,
	}
}

// MapPayload transforms the input data according to the configured mapping
func (pm *PayloadMapper) MapPayload(data interface{}) map[string]interface{} {
	if pm.mapping == nil {
		return nil
	}

	result := make(map[string]interface{})
	pm.processMapping(pm.mapping, result, data)
	return result
}

// processMapping recursively processes the mapping configuration
func (pm *PayloadMapper) processMapping(mapping map[string]interface{}, result map[string]interface{}, data interface{}) {
	for key, value := range mapping {
		switch v := value.(type) {
		case string:
			// Check if string contains template syntax
			if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
				processedValue := pm.processTemplate(v, data)
				if !pm.omitEmpty || processedValue != "" {
					result[key] = processedValue
				}
			} else if !pm.omitEmpty || v != "" {
				result[key] = v
			}
		case map[string]interface{}:
			// Handle nested objects
			nestedResult := make(map[string]interface{})
			pm.processMapping(v, nestedResult, data)
			// Only include nested objects if they have values or omitEmpty is false
			if !pm.omitEmpty || len(nestedResult) > 0 {
				result[key] = nestedResult
			}
		default:
			// Static values (numbers, booleans, etc.)
			result[key] = value
		}
	}
}

// processTemplate executes a template string with the provided data
func (pm *PayloadMapper) processTemplate(templateString string, data interface{}) string {
	// Create template with custom functions
	template, err := template.New("payload").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"formatTimePtr": func(t *time.Time) string {
			if t != nil {
				return t.Format(time.RFC3339)
			}
			return ""
		},
		// Add more helper functions as needed
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"trim":  strings.TrimSpace,
	}).Parse(templateString)

	if err != nil {
		slog.Error("Failed to parse template", "error", err, "template", templateString)
		return templateString
	}

	// Get buffer from pool
	templateBuffer := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		templateBuffer.Reset()
		bufferPool.Put(templateBuffer)
	}()

	// Execute template
	if err := template.Execute(templateBuffer, data); err != nil {
		slog.Error("Failed to execute template", "error", err, "template", templateString)
		return templateString
	}

	return templateBuffer.String()
}
