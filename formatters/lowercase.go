package formatters

import (
	"strings"

	"zwfm-metadata/core"
)

// LowercaseFormatter converts artist and title to lowercase.
type LowercaseFormatter struct{}

// Format transforms artist and title fields to lowercase.
func (l *LowercaseFormatter) Format(st *core.StructuredText) {
	st.Artist = strings.ToLower(st.Artist)
	st.Title = strings.ToLower(st.Title)
}

func init() {
	RegisterFormatter("lowercase", func() core.Formatter { return &LowercaseFormatter{} })
}
