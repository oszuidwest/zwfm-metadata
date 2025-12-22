package formatters

import "strings"

// UppercaseFormatter converts all characters to uppercase.
type UppercaseFormatter struct{}

// Format returns text in uppercase.
func (u *UppercaseFormatter) Format(text string) string {
	return strings.ToUpper(text)
}

func init() {
	RegisterFormatter("uppercase", func() Formatter { return &UppercaseFormatter{} })
}
