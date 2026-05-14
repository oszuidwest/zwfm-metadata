package utils

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
			},
		},
	}
	pm := NewPayloadMapper(mapping)
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
	if len(items) != 1 {
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
}
