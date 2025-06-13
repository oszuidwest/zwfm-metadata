package formatters

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// UcwordsFormatter capitalizes the first letter of each word
type UcwordsFormatter struct{}

// Format implements the Formatter interface
func (u *UcwordsFormatter) Format(text string) string {
	caser := cases.Title(language.English)
	return caser.String(strings.ToLower(text))
}

func init() {
	RegisterFormatter("ucwords", func() Formatter { return &UcwordsFormatter{} })
}
