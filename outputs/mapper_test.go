package outputs

import (
	"testing"
	"time"
)

func TestPayloadMapper_ArrayOfMapsExpandsTemplates(t *testing.T) {
	mapping := map[string]any{
		"event": map[string]any{
			"items": []any{
				map[string]any{
					"track":     "{{.title}}",
					"performer": "{{.artist}}",
					"logged_at": "{{.updated_at}}",
				},
				map[string]any{
					"summary": "{{.artist}} - {{.title}}",
				},
			},
		},
	}
	pm, err := NewPayloadMapper(mapping)
	if err != nil {
		t.Fatalf("NewPayloadMapper() error = %v", err)
	}
	data := map[string]any{
		"title":      "Test Song",
		"artist":     "Test Artist",
		"updated_at": time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}
	got := pm.MapPayload(data)

	event, ok := got["event"].(map[string]any)
	if !ok {
		t.Fatalf("event: got %T", got["event"])
	}
	items, ok := event["items"].([]any)
	if !ok {
		t.Fatalf("items: got %T", event["items"])
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d", len(items))
	}
	row, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("items[0]: got %T", items[0])
	}
	if row["track"] != "Test Song" {
		t.Errorf("track = %q", row["track"])
	}
	if row["performer"] != "Test Artist" {
		t.Errorf("performer = %q", row["performer"])
	}
	if row["logged_at"] != "2026-05-14T12:00:00Z" {
		t.Errorf("logged_at = %q", row["logged_at"])
	}
	secondRow, ok := items[1].(map[string]any)
	if !ok {
		t.Fatalf("items[1]: got %T", items[1])
	}
	if secondRow["summary"] != "Test Artist - Test Song" {
		t.Errorf("summary = %q", secondRow["summary"])
	}
}

func TestPayloadMapper_ArrayPrimitiveValuesPassThrough(t *testing.T) {
	mapping := map[string]any{
		"items": []any{
			"static text",
			"{{.title}}",
			float64(123),
			true,
			nil,
		},
	}
	pm, err := NewPayloadMapper(mapping)
	if err != nil {
		t.Fatalf("NewPayloadMapper() error = %v", err)
	}
	got := pm.MapPayload(map[string]any{"title": "Test Song"})

	items, ok := got["items"].([]any)
	if !ok {
		t.Fatalf("items: got %T", got["items"])
	}
	if len(items) != 5 {
		t.Fatalf("len(items) = %d", len(items))
	}
	if items[0] != "static text" {
		t.Errorf("items[0] = %q", items[0])
	}
	if items[1] != "{{.title}}" {
		t.Errorf("items[1] = %q", items[1])
	}
	if items[2] != float64(123) {
		t.Errorf("items[2] = %v", items[2])
	}
	if items[3] != true {
		t.Errorf("items[3] = %v", items[3])
	}
	if items[4] != nil {
		t.Errorf("items[4] = %v", items[4])
	}
}

func TestNewPayloadMapperRejectsInvalidTemplate(t *testing.T) {
	_, err := NewPayloadMapper(map[string]any{"title": "{{"})
	if err == nil {
		t.Fatal("NewPayloadMapper() error = nil, want invalid template error")
	}
}
