package core

import "testing"

func TestNewStructuredTextWithNilMetadata(t *testing.T) {
	got := NewStructuredText(nil)
	want := StructuredText{Separator: " - "}

	if got == nil {
		t.Fatal("NewStructuredText(nil) returned nil")
	}
	if *got != want {
		t.Errorf("NewStructuredText(nil) = %#v, want %#v", *got, want)
	}
}
