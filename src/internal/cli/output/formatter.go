// Package output provides output formatting for tokmesh-cli.
package output

import "io"

// Format represents the output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Formatter formats data for output.
type Formatter interface {
	Format(w io.Writer, data any) error
}

// NewFormatter creates a formatter for the given format.
func NewFormatter(format Format, wide bool) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{}
	case FormatYAML:
		return &YAMLFormatter{}
	default:
		return &TableFormatter{Wide: wide}
	}
}
