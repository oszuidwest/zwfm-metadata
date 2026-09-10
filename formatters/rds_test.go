package formatters

import (
	"strings"
	"testing"

	"zwfm-metadata/core"
)

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
