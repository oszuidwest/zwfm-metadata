// Package formatters transforms metadata for output protocols.
package formatters

import (
	"fmt"

	"zwfm-metadata/core"
)

// New creates the formatter with the given name.
func New(name string) (core.Formatter, error) {
	switch name {
	case "lowercase":
		return &LowercaseFormatter{}, nil
	case "uppercase":
		return &UppercaseFormatter{}, nil
	case "ucwords":
		return &UcwordsFormatter{}, nil
	case "rds":
		return &RDSFormatter{}, nil
	default:
		return nil, fmt.Errorf("unknown formatter: %s", name)
	}
}
