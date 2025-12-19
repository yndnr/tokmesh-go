// Package output provides output formatting for tokmesh-cli.
package output

import (
	"encoding/json"
	"io"
)

// JSONFormatter formats data as JSON.
type JSONFormatter struct{}

// Format formats data as indented JSON.
func (f *JSONFormatter) Format(w io.Writer, data any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
