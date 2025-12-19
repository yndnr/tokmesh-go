// Package output provides output formatting for tokmesh-cli.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
	"time"
)

// TableFormatter formats data as an ASCII table.
type TableFormatter struct {
	Wide      bool
	NoHeaders bool
}

// Format formats data as a table.
// Supports: Table, []T (slice of structs/maps), map[string]any
func (f *TableFormatter) Format(w io.Writer, data any) error {
	if data == nil {
		return nil
	}

	// If data is already a Table, render it directly
	if t, ok := data.(*Table); ok {
		return t.RenderWithOptions(w, f.NoHeaders)
	}
	if t, ok := data.(Table); ok {
		return t.RenderWithOptions(w, f.NoHeaders)
	}

	// Try to convert to table
	table, err := toTable(data, f.Wide)
	if err != nil {
		// Fallback to JSON for complex types
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}

	return table.RenderWithOptions(w, f.NoHeaders)
}

// toTable converts various data types to a Table.
func toTable(data any, wide bool) (*Table, error) {
	v := reflect.ValueOf(data)

	// Dereference pointer
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return sliceToTable(v, wide)
	case reflect.Map:
		return mapToTable(v)
	case reflect.Struct:
		return structToTable(v)
	default:
		return nil, fmt.Errorf("unsupported type: %s", v.Kind())
	}
}

// sliceToTable converts a slice to a table.
func sliceToTable(v reflect.Value, wide bool) (*Table, error) {
	if v.Len() == 0 {
		return &Table{}, nil
	}

	// Get headers from first element
	first := v.Index(0)
	if first.Kind() == reflect.Ptr {
		first = first.Elem()
	}

	var headers []string
	var fieldIndices []int

	switch first.Kind() {
	case reflect.Struct:
		t := first.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if !field.IsExported() {
				continue
			}
			// Skip wide-only fields if not in wide mode
			tag := field.Tag.Get("table")
			if tag == "-" {
				continue
			}
			if strings.Contains(tag, "wide") && !wide {
				continue
			}
			// Use json tag for header name if available
			name := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" && parts[0] != "-" {
					name = parts[0]
				}
			}
			headers = append(headers, strings.ToUpper(toSnakeCase(name)))
			fieldIndices = append(fieldIndices, i)
		}
	case reflect.Map:
		// For maps, use keys as rows
		headers = []string{"KEY", "VALUE"}
	default:
		headers = []string{"VALUE"}
	}

	table := &Table{Headers: headers}

	// Build rows
	for i := 0; i < v.Len(); i++ {
		elem := v.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}

		var row []string
		switch elem.Kind() {
		case reflect.Struct:
			for _, idx := range fieldIndices {
				row = append(row, formatValue(elem.Field(idx)))
			}
		case reflect.Map:
			iter := elem.MapRange()
			for iter.Next() {
				row = []string{formatValue(iter.Key()), formatValue(iter.Value())}
				table.Rows = append(table.Rows, row)
			}
			continue
		default:
			row = []string{formatValue(elem)}
		}
		table.Rows = append(table.Rows, row)
	}

	return table, nil
}

// mapToTable converts a map to a key-value table.
func mapToTable(v reflect.Value) (*Table, error) {
	table := &Table{
		Headers: []string{"KEY", "VALUE"},
	}

	iter := v.MapRange()
	for iter.Next() {
		key := formatValue(iter.Key())
		val := formatValue(iter.Value())
		table.Rows = append(table.Rows, []string{key, val})
	}

	return table, nil
}

// structToTable converts a single struct to a key-value table.
func structToTable(v reflect.Value) (*Table, error) {
	table := &Table{
		Headers: []string{"FIELD", "VALUE"},
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		name := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" && parts[0] != "-" {
				name = parts[0]
			}
		}

		val := formatValue(v.Field(i))
		table.Rows = append(table.Rows, []string{name, val})
	}

	return table, nil
}

// formatValue formats a reflect.Value for display.
func formatValue(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}

	// Handle interface
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// Handle pointer
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	// Handle time.Time specially
	if v.Type() == reflect.TypeOf(time.Time{}) {
		t := v.Interface().(time.Time)
		if t.IsZero() {
			return "-"
		}
		return t.Format("2006-01-02 15:04")
	}

	// Handle common types
	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if s == "" {
			return "-"
		}
		return s
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", v.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%.2f", v.Float())
	case reflect.Bool:
		if v.Bool() {
			return "true"
		}
		return "false"
	case reflect.Slice, reflect.Array:
		if v.Len() == 0 {
			return "-"
		}
		return fmt.Sprintf("[%d items]", v.Len())
	case reflect.Map:
		if v.Len() == 0 {
			return "-"
		}
		return fmt.Sprintf("{%d keys}", v.Len())
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

// toSnakeCase converts CamelCase to SNAKE_CASE.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return result.String()
}

// Table represents tabular data.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Render renders the table to the writer.
func (t *Table) Render(w io.Writer) error {
	return t.RenderWithOptions(w, false)
}

// RenderWithOptions renders the table with options.
func (t *Table) RenderWithOptions(w io.Writer, noHeaders bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Write headers
	if !noHeaders && len(t.Headers) > 0 {
		for i, h := range t.Headers {
			if i > 0 {
				tw.Write([]byte("\t"))
			}
			tw.Write([]byte(h))
		}
		tw.Write([]byte("\n"))
	}

	// Write rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i > 0 {
				tw.Write([]byte("\t"))
			}
			tw.Write([]byte(cell))
		}
		tw.Write([]byte("\n"))
	}

	return nil
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cells ...string) {
	t.Rows = append(t.Rows, cells)
}

// SetHeaders sets the table headers.
func (t *Table) SetHeaders(headers ...string) {
	t.Headers = headers
}
