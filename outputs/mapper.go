package outputs

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

// TemplateFuncs is the shared function set available in all metadata templates.
var TemplateFuncs = template.FuncMap{
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
}

// PayloadMapper handles custom payload transformation based on configuration.
type PayloadMapper struct {
	mapping   map[string]any
	templates map[string]*template.Template // template string -> pre-compiled template (nil if parsing failed)
}

// NewPayloadMapper returns a new PayloadMapper with all template strings in the mapping pre-compiled.
func NewPayloadMapper(mapping map[string]any) *PayloadMapper {
	pm := &PayloadMapper{
		mapping:   mapping,
		templates: make(map[string]*template.Template),
	}
	pm.compileTemplates(mapping)
	return pm
}

// compileTemplates walks the mapping tree and pre-compiles every template string it contains.
func (pm *PayloadMapper) compileTemplates(value any) {
	switch v := value.(type) {
	case string:
		if !isTemplate(v) {
			return
		}
		if _, seen := pm.templates[v]; seen {
			return
		}
		tmpl, err := template.New("payload").Funcs(TemplateFuncs).Parse(v)
		if err != nil {
			slog.Error("Failed to parse template", "error", err, "template", v)
			tmpl = nil // remembered as broken; processTemplate falls back to the raw string
		}
		pm.templates[v] = tmpl
	case map[string]any:
		for _, item := range v {
			pm.compileTemplates(item)
		}
	default:
		if items, ok := asAnySlice(v); ok {
			for _, item := range items {
				pm.compileTemplates(item)
			}
		}
	}
}

// asAnySlice normalizes a slice of any element type to []any; JSON-decoded
// mappings already arrive as []any, but programmatic callers may pass typed
// slices like []map[string]any.
func asAnySlice(value any) ([]any, bool) {
	if items, ok := value.([]any); ok {
		return items, true
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() != reflect.Slice {
		return nil, false
	}
	items := make([]any, rv.Len())
	for i := range rv.Len() {
		items[i] = rv.Index(i).Interface()
	}
	return items, true
}

// isTemplate reports whether a mapping string contains template syntax.
func isTemplate(s string) bool {
	return strings.Contains(s, "{{") && strings.Contains(s, "}}")
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

// processMapping walks the mapping tree and applies templates to strings, nested maps, and object slices.
func (pm *PayloadMapper) processMapping(mapping, result map[string]any, data any) {
	for key, value := range mapping {
		switch v := value.(type) {
		case string:
			if isTemplate(v) {
				result[key] = pm.processTemplate(v, data)
			} else {
				result[key] = v
			}
		case map[string]any:
			nestedResult := make(map[string]any)
			pm.processMapping(v, nestedResult, data)
			result[key] = nestedResult
		default:
			if items, ok := asAnySlice(value); ok {
				result[key] = pm.processMappingSlice(items, data)
			} else {
				result[key] = value
			}
		}
	}
}

// processMappingSlice expands templates inside array-of-object mappings; non-object elements are copied as-is.
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

// processTemplate executes a pre-compiled template with the provided data.
func (pm *PayloadMapper) processTemplate(templateString string, data any) string {
	tmpl := pm.templates[templateString]
	if tmpl == nil {
		// Parsing failed at construction (already logged); fall back to the raw string.
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
