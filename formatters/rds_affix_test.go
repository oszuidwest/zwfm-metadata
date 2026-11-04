package formatters

import (
	"testing"

	"zwfm-metadata/core"
)

func TestRDSFormatterCleansPrefixAndSuffix(t *testing.T) {
	tests := []struct {
		name       string
		prefix     string
		suffix     string
		wantPrefix string
		wantSuffix string
	}{
		{name: "keeps outer spaces", prefix: "Nu: ", suffix: " [Live]", wantPrefix: "Nu: ", wantSuffix: " [Live]"},
		{name: "keeps a tab as a space", prefix: "Nu:\t", suffix: "\t[Live]", wantPrefix: "Nu: ", wantSuffix: " [Live]"},
		{name: "removes an emoji", prefix: "\U0001F3B5 Now Playing: ", suffix: " \U0001F3B5 end", wantPrefix: "Now Playing: ", wantSuffix: " end"},
		{name: "drops an affix without text", prefix: "\U0001F3B5 ", suffix: " \U0001F3B5", wantPrefix: "", wantSuffix: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := core.NewStructuredText(&core.Metadata{Title: "Song"})
			st.Prefix = tt.prefix
			st.Suffix = tt.suffix
			new(RDSFormatter).Format(st)
			if st.Prefix != tt.wantPrefix || st.Suffix != tt.wantSuffix {
				t.Errorf("Prefix, Suffix = %q, %q, want %q, %q", st.Prefix, st.Suffix, tt.wantPrefix, tt.wantSuffix)
			}
		})
	}
}
