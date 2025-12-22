package formatters

import "strings"

// LowercaseFormatter converts all characters to lowercase.
type LowercaseFormatter struct{}

// Format returns text in lowercase.
func (l *LowercaseFormatter) Format(text string) string {
	return strings.ToLower(text)
}

func init() {
	RegisterFormatter("lowercase", func() Formatter { return &LowercaseFormatter{} })
}
