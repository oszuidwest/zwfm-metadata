package utils

import (
	"bytes"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"time"
)

// bufferPool reduces allocations during template processing.
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// PayloadMapper handles custom payload transformation based on configuration.
type PayloadMapper struct {
	mapping map[string]any
}

// NewPayloadMapper returns a new PayloadMapper with the given mapping configuration.
func NewPayloadMapper(mapping map[string]any) *PayloadMapper {
	return &PayloadMapper{
		mapping: mapping,
	}
}

// MapPayload transforms the input data according to the configured mapping.
func (pm *PayloadMapper) MapPayload(data any) map[string]any {
	if pm.mapping == nil {
		return nil
	}

	result := make(map[string]any)
	pm.processMapping(pm.mapping, result, data)
	return result
}

// processMapping walks the mapping tree and applies templates to string values.
func (pm *PayloadMapper) processMapping(mapping, result map[string]any, data any) {
	for key, value := range mapping {
		switch v := value.(type) {
		case string:
			if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
				processedValue := pm.processTemplate(v, data)
				result[key] = processedValue
			} else {
				result[key] = v
			}
		case map[string]any:
			nestedResult := make(map[string]any)
			pm.processMapping(v, nestedResult, data)
			result[key] = nestedResult
		default:
			if rv := reflect.ValueOf(value); rv.Kind() == reflect.Slice {
				items := make([]any, rv.Len())
				for i := range rv.Len() {
					items[i] = rv.Index(i).Interface()
				}
				result[key] = pm.processMappingSlice(items, data)
			} else {
				result[key] = value
			}
		}
	}
}

// processMappingSlice applies template mapping to each object in a JSON array (other elements pass through).
func (pm *PayloadMapper) processMappingSlice(items []any, data any) []any {
	out := make([]any, len(items))
	for i, item := range items {
		nested, ok := item.(map[string]any)
		if !ok {
			out[i] = item
			continue
		}
		nestedResult := make(map[string]any)
		pm.processMapping(nested, nestedResult, data)
		out[i] = nestedResult
	}
	return out
}

// processTemplate executes a template string with the provided data.
func (pm *PayloadMapper) processTemplate(templateString string, data any) string {
	tmpl, err := template.New("payload").Funcs(template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format(time.RFC3339)
		},
		"formatTimePtr": func(t *time.Time) string {
			if t != nil {
				return t.Format(time.RFC3339)
			}
			return ""
		},
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
		"trim":  strings.TrimSpace,
	}).Parse(templateString)

	if err != nil {
		slog.Error("Failed to parse template", "error", err, "template", templateString)
		return templateString
	}

	templateBuffer := bufferPool.Get().(*bytes.Buffer)
	defer func() {
		templateBuffer.Reset()
		bufferPool.Put(templateBuffer)
	}()

	if err := tmpl.Execute(templateBuffer, data); err != nil {
		slog.Error("Failed to execute template", "error", err, "template", templateString)
		return templateString
	}

	return templateBuffer.String()
}
