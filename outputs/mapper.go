package outputs

import (
	"fmt"
	"log/slog"
	"strings"
	"text/template"
)

var templateFuncs = template.FuncMap{
	"lower": strings.ToLower,
	"upper": strings.ToUpper,
	"trim":  strings.TrimSpace,
}

// PayloadMapper transforms metadata into a configured payload shape. Mapping values
// come from JSON config: strings (optionally templates), objects, and arrays.
type PayloadMapper struct {
	mapping   map[string]any
	templates map[string]*template.Template
}

// NewPayloadMapper compiles every template string in the mapping. A nil mapping
// yields a nil mapper, which Apply treats as "no mapping".
func NewPayloadMapper(mapping map[string]any) (*PayloadMapper, error) {
	if mapping == nil {
		return nil, nil
	}

	pm := &PayloadMapper{
		mapping:   mapping,
		templates: make(map[string]*template.Template),
	}
	if err := pm.compileTemplates(mapping); err != nil {
		return nil, err
	}
	return pm, nil
}

func (pm *PayloadMapper) compileTemplates(value any) error {
	switch v := value.(type) {
	case string:
		if !isTemplate(v) {
			return nil
		}
		if _, seen := pm.templates[v]; seen {
			return nil
		}
		tmpl, err := template.New("payload").Funcs(templateFuncs).Parse(v)
		if err != nil {
			return fmt.Errorf("invalid payload template %q: %w", v, err)
		}
		pm.templates[v] = tmpl
	case map[string]any:
		for _, item := range v {
			if err := pm.compileTemplates(item); err != nil {
				return err
			}
		}
	case []any:
		// Only objects inside arrays are expanded; other elements are copied as-is.
		for _, item := range v {
			if nested, ok := item.(map[string]any); ok {
				if err := pm.compileTemplates(nested); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func isTemplate(s string) bool {
	return strings.Contains(s, "{{")
}

// Apply returns the mapped payload for um, or um itself when there is no mapping.
func (pm *PayloadMapper) Apply(um *UniversalMetadata) any {
	if pm == nil {
		return um
	}
	return pm.MapPayload(um.ToTemplateData())
}

// MapPayload transforms the input data according to the configured mapping.
func (pm *PayloadMapper) MapPayload(data any) map[string]any {
	return pm.processMapping(pm.mapping, data)
}

func (pm *PayloadMapper) processMapping(mapping map[string]any, data any) map[string]any {
	result := make(map[string]any, len(mapping))
	for key, value := range mapping {
		switch v := value.(type) {
		case string:
			result[key] = pm.processTemplate(v, data)
		case map[string]any:
			result[key] = pm.processMapping(v, data)
		case []any:
			result[key] = pm.processMappingSlice(v, data)
		default:
			result[key] = value
		}
	}
	return result
}

func (pm *PayloadMapper) processMappingSlice(items []any, data any) []any {
	out := make([]any, len(items))
	for i, item := range items {
		if nested, ok := item.(map[string]any); ok {
			out[i] = pm.processMapping(nested, data)
		} else {
			out[i] = item
		}
	}
	return out
}

func (pm *PayloadMapper) processTemplate(s string, data any) string {
	tmpl := pm.templates[s]
	if tmpl == nil {
		return s
	}

	var b strings.Builder
	if err := tmpl.Execute(&b, data); err != nil {
		slog.Error("Failed to execute template", "template", s, "error", err)
		return s
	}
	return b.String()
}
