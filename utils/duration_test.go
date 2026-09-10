package utils

import "testing"

func TestParseDurationToSeconds(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"272", 272, true},
		{"272.5", 273, true},
		{"272,4", 272, true},
		{" 90 ", 90, true},
		{"3:45", 225, true},
		{"1:30:00", 5400, true},
		{"abc", 0, false},
		{"3:60", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParseDurationToSeconds(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ParseDurationToSeconds(%q) = (%d, %v), want (%d, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
