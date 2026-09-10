package outputs

import "testing"

func TestConvertStructuredText_Nil(t *testing.T) {
	got := ConvertStructuredText(nil)
	if got == nil {
		t.Fatal("ConvertStructuredText(nil) = nil")
	}
	if got.UpdatedAt.IsZero() {
		t.Error("ConvertStructuredText(nil).UpdatedAt is zero")
	}
}
