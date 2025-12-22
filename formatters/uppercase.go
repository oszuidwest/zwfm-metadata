package formatters

import (
	"strings"

	"zwfm-metadata/core"
)

// UppercaseFormatter converts artist and title to uppercase.
type UppercaseFormatter struct{}

// Format transforms artist and title fields to uppercase.
func (u *UppercaseFormatter) Format(st *core.StructuredText) {
	st.Artist = strings.ToUpper(st.Artist)
	st.Title = strings.ToUpper(st.Title)
}

func init() {
	RegisterFormatter("uppercase", func() core.Formatter { return &UppercaseFormatter{} })
}
