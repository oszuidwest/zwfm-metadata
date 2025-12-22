package formatters

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"zwfm-metadata/core"
)

// UcwordsFormatter capitalizes the first letter of each word (title case).
type UcwordsFormatter struct{}

// Format transforms artist and title fields to title case.
func (u *UcwordsFormatter) Format(st *core.StructuredText) {
	caser := cases.Title(language.English)
	st.Artist = caser.String(strings.ToLower(st.Artist))
	st.Title = caser.String(strings.ToLower(st.Title))
}

func init() {
	RegisterFormatter("ucwords", func() core.Formatter { return &UcwordsFormatter{} })
}
