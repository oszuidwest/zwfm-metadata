package formatters

import (
	"strings"
	"testing"

	"zwfm-metadata/core"
)

func TestRDSFormatterDigraphCase(t *testing.T) {
	st := core.NewStructuredText(&core.Metadata{Title: "\u01c8 \u01c9 \u01cb \u01cc \u01c5 \u01c6"})
	new(RDSFormatter).Format(st)
	if want := "Lj lj Nj nj Dz dz"; st.Title != want {
		t.Errorf("Title = %q, want %q", st.Title, want)
	}
}

func TestRDSFormatterOverBudget(t *testing.T) {
	const artist = "Primary Artist"
	const title = "A Moderately Long Song Title For Radio"

	tests := []struct {
		name       string
		artist     string
		title      string
		wantArtist string
		wantTitle  string
	}{
		{
			name:       "remove title parentheses",
			artist:     artist,
			title:      title + " (Recorded Live at Wembley Stadium)",
			wantArtist: artist,
			wantTitle:  title,
		},
		{
			name:       "remove featured artist",
			artist:     artist + " featuring Guest Artist and Orchestra",
			title:      title,
			wantArtist: artist,
			wantTitle:  title,
		},
		{
			name:       "remove remix suffix",
			artist:     artist,
			title:      title + " - Twelve Inch Extended Radio Remix",
			wantArtist: artist,
			wantTitle:  title,
		},
		{
			name:       "truncate both fields",
			artist:     strings.Repeat("A", 50),
			title:      strings.Repeat("B", 50),
			wantArtist: strings.Repeat("A", 27) + "...",
			wantTitle:  strings.Repeat("B", 27) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := core.NewStructuredText(&core.Metadata{Artist: tt.artist, Title: tt.title})
			if st.Len() <= maxRDSLength {
				t.Fatalf("test input length = %d, want over %d", st.Len(), maxRDSLength)
			}

			new(RDSFormatter).Format(st)

			if st.Artist != tt.wantArtist || st.Title != tt.wantTitle {
				t.Errorf("Format() = %q / %q, want %q / %q", st.Artist, st.Title, tt.wantArtist, tt.wantTitle)
			}
			if st.Len() > maxRDSLength {
				t.Errorf("formatted length = %d, want at most %d", st.Len(), maxRDSLength)
			}
		})
	}
}

func TestRDSFormatterKeepsPunctuationReadable(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Don\u2019t \u2018Stop\u2019 \u201aa\u201b", want: "Don't 'Stop' 'a'"},
		{in: "\u201cHello\u201d \u201eWorld\u201d", want: `"Hello" "World"`},
		{in: "Now \u2013 Then \u2014 Later \u2010a \u2011b \u2015c", want: "Now - Then - Later -a -b -c"},
		{in: "Wait\u2026", want: "Wait..."},
		{in: "\u20ac5", want: "EUR5"},
		{in: "a\u00a0b\u2002c\u2003d\u2009e\u200af\u202fg", want: "a b c d e f g"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			st := core.NewStructuredText(&core.Metadata{Title: tt.in})
			new(RDSFormatter).Format(st)
			if st.Title != tt.want {
				t.Errorf("Title = %q, want %q", st.Title, tt.want)
			}
		})
	}
}
