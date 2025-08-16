package formatters

import "strings"

// LowercaseFormatter converts text to lowercase.
type LowercaseFormatter struct{}

// Format implements the Formatter interface.
func (l *LowercaseFormatter) Format(text string) string {
	return strings.ToLower(text)
}

func init() {
	RegisterFormatter("lowercase", func() Formatter { return &LowercaseFormatter{} })
}
