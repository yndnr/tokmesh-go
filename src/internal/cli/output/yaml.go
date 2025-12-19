// Package output provides output formatting for tokmesh-cli.
package output

import "io"

// YAMLFormatter formats data as YAML.
type YAMLFormatter struct{}

// Format formats data as YAML.
func (f *YAMLFormatter) Format(w io.Writer, data any) error {
	// TODO: Use gopkg.in/yaml.v3 for YAML encoding
	return nil
}
