package formatters

import "strings"

// UppercaseFormatter converts text to uppercase.
type UppercaseFormatter struct{}

// Format implements the Formatter interface.
func (u *UppercaseFormatter) Format(text string) string {
	return strings.ToUpper(text)
}

func init() {
	RegisterFormatter("uppercase", func() Formatter { return &UppercaseFormatter{} })
}
