package formatters

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// UcwordsFormatter capitalizes the first letter of each word (title case).
type UcwordsFormatter struct{}

// Format returns text with title case applied.
func (u *UcwordsFormatter) Format(text string) string {
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(text))
}

func init() {
	RegisterFormatter("ucwords", func() Formatter { return &UcwordsFormatter{} })
}
