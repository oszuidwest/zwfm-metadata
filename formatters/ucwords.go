package formatters

import (
	"strings"
	"unicode"

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

// Alternative implementation that handles edge cases better
func (u *UcwordsFormatter) FormatV2(text string) string {
	// Convert to rune slice for proper Unicode handling
	runes := []rune(text)
	inWord := false

	for i, r := range runes {
		if unicode.IsLetter(r) {
			if !inWord {
				runes[i] = unicode.ToUpper(r)
				inWord = true
			} else {
				runes[i] = unicode.ToLower(r)
			}
		} else {
			inWord = false
		}
	}

	return string(runes)
}

func init() {
	RegisterFormatter("ucwords", func() Formatter { return &UcwordsFormatter{} })
}
