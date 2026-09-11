package utils

import "testing"

func TestParseDurationToSeconds(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   int
		wantOK bool
	}{
		{name: "whole seconds", input: "272", want: 272, wantOK: true},
		{name: "decimal point", input: "272.5", want: 273, wantOK: true},
		{name: "decimal comma", input: "272,4", want: 272, wantOK: true},
		{name: "minutes and seconds", input: "3:45", want: 225, wantOK: true},
		{name: "hours minutes and seconds", input: "1:30:00", want: 5400, wantOK: true},
		{name: "non-numeric", input: "abc", want: 0, wantOK: false},
		{name: "seconds out of range", input: "3:60", want: 0, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseDurationToSeconds(tt.input)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("ParseDurationToSeconds(%q) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
