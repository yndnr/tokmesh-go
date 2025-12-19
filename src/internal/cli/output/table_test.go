package output

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTableFormatter_Format_Table(t *testing.T) {
	table := &Table{
		Headers: []string{"NAME", "VALUE"},
		Rows: [][]string{
			{"key1", "value1"},
			{"key2", "value2"},
		},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, table)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("Format() missing header NAME")
	}
	if !strings.Contains(output, "key1") {
		t.Error("Format() missing row data key1")
	}
}

func TestTableFormatter_Format_TableValue(t *testing.T) {
	// Test with Table (not pointer)
	table := Table{
		Headers: []string{"COL"},
		Rows:    [][]string{{"data"}},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, table)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	if !strings.Contains(buf.String(), "data") {
		t.Error("Format() missing data from Table value")
	}
}

func TestTableFormatter_Format_TableNoHeaders(t *testing.T) {
	table := &Table{
		Headers: []string{"NAME", "VALUE"},
		Rows: [][]string{
			{"key1", "value1"},
		},
	}

	var buf bytes.Buffer
	f := &TableFormatter{NoHeaders: true}

	err := f.Format(&buf, table)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if strings.Contains(output, "NAME") {
		t.Error("Format() should not contain headers when NoHeaders=true")
	}
	if !strings.Contains(output, "key1") {
		t.Error("Format() missing row data")
	}
}

func TestTableFormatter_Format_Nil(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, nil)
	if err != nil {
		t.Fatalf("Format(nil) error = %v", err)
	}

	if buf.Len() != 0 {
		t.Error("Format(nil) should produce empty output")
	}
}

type testStruct struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Active  bool   `json:"active"`
	Details string `json:"details" table:"wide"`
}

func TestTableFormatter_Format_Slice(t *testing.T) {
	data := []testStruct{
		{Name: "Alice", Age: 30, Active: true, Details: "detail1"},
		{Name: "Bob", Age: 25, Active: false, Details: "detail2"},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Error("Format() missing header")
	}
	if !strings.Contains(output, "Alice") {
		t.Error("Format() missing row data")
	}
	if !strings.Contains(output, "30") {
		t.Error("Format() missing age data")
	}
	// Wide-only field should not appear
	if strings.Contains(output, "DETAILS") {
		t.Error("Format() should not include wide-only field when Wide=false")
	}
}

func TestTableFormatter_Format_SliceWide(t *testing.T) {
	data := []testStruct{
		{Name: "Alice", Age: 30, Active: true, Details: "detail1"},
	}

	var buf bytes.Buffer
	f := &TableFormatter{Wide: true}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	// Wide-only field should appear when Wide=true
	if !strings.Contains(output, "DETAILS") {
		t.Error("Format() should include wide-only field when Wide=true")
	}
	if !strings.Contains(output, "detail1") {
		t.Error("Format() missing wide field data")
	}
}

func TestTableFormatter_Format_EmptySlice(t *testing.T) {
	var data []testStruct

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Empty slice should produce minimal output
	output := buf.String()
	if strings.Contains(output, "NAME") {
		t.Error("Format() should not have headers for empty slice")
	}
}

func TestTableFormatter_Format_Map(t *testing.T) {
	data := map[string]any{
		"key1": "value1",
		"key2": 42,
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "KEY") || !strings.Contains(output, "VALUE") {
		t.Error("Format() missing map headers")
	}
}

type singleStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

func TestTableFormatter_Format_SingleStruct(t *testing.T) {
	data := singleStruct{Field1: "test", Field2: 123}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "FIELD") || !strings.Contains(output, "VALUE") {
		t.Error("Format() missing struct headers")
	}
	if !strings.Contains(output, "test") || !strings.Contains(output, "123") {
		t.Error("Format() missing struct data")
	}
}

func TestTableFormatter_Format_PointerSlice(t *testing.T) {
	data := []*testStruct{
		{Name: "Alice", Age: 30},
		{Name: "Bob", Age: 25},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Alice") || !strings.Contains(output, "Bob") {
		t.Error("Format() missing pointer slice data")
	}
}

func TestTable_Render(t *testing.T) {
	table := &Table{
		Headers: []string{"COL1", "COL2"},
		Rows: [][]string{
			{"a", "b"},
			{"c", "d"},
		},
	}

	var buf bytes.Buffer
	err := table.Render(&buf)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 { // 1 header + 2 data rows
		t.Errorf("Render() lines = %d, want 3", len(lines))
	}
}

func TestTable_RenderWithOptions_NoRows(t *testing.T) {
	table := &Table{
		Headers: []string{"COL1", "COL2"},
		Rows:    [][]string{},
	}

	var buf bytes.Buffer
	err := table.RenderWithOptions(&buf, false)
	if err != nil {
		t.Fatalf("RenderWithOptions() error = %v", err)
	}

	// Should still have headers
	if !strings.Contains(buf.String(), "COL1") {
		t.Error("RenderWithOptions() missing headers")
	}
}

func TestTable_AddRow(t *testing.T) {
	table := &Table{}
	table.AddRow("cell1", "cell2", "cell3")

	if len(table.Rows) != 1 {
		t.Errorf("AddRow() rows = %d, want 1", len(table.Rows))
	}
	if len(table.Rows[0]) != 3 {
		t.Errorf("AddRow() cols = %d, want 3", len(table.Rows[0]))
	}
}

func TestTable_SetHeaders(t *testing.T) {
	table := &Table{}
	table.SetHeaders("H1", "H2", "H3")

	if len(table.Headers) != 3 {
		t.Errorf("SetHeaders() headers = %d, want 3", len(table.Headers))
	}
	if table.Headers[0] != "H1" {
		t.Errorf("SetHeaders() first header = %s, want H1", table.Headers[0])
	}
}

func TestFormatValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"empty string", "", "-"},
		{"int", 42, "42"},
		{"int64", int64(123), "123"},
		{"uint", uint(99), "99"},
		{"float64", 3.14159, "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"empty slice", []int{}, "-"},
		{"slice", []int{1, 2, 3}, "[3 items]"},
		{"empty map", map[string]int{}, "-"},
		{"map", map[string]int{"a": 1}, "{1 keys}"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatValue(reflect.ValueOf(tc.input))
			if result != tc.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatValue_Time(t *testing.T) {
	// Non-zero time
	tm := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	result := formatValue(reflect.ValueOf(tm))
	if result != "2024-06-15 14:30" {
		t.Errorf("formatValue(time) = %q, want %q", result, "2024-06-15 14:30")
	}

	// Zero time
	var zeroTime time.Time
	result = formatValue(reflect.ValueOf(zeroTime))
	if result != "-" {
		t.Errorf("formatValue(zero time) = %q, want %q", result, "-")
	}
}

func TestFormatValue_Pointer(t *testing.T) {
	val := "pointer value"
	result := formatValue(reflect.ValueOf(&val))
	if result != "pointer value" {
		t.Errorf("formatValue(*string) = %q, want %q", result, "pointer value")
	}

	var nilPtr *string
	result = formatValue(reflect.ValueOf(nilPtr))
	if result != "" {
		t.Errorf("formatValue(nil ptr) = %q, want empty", result)
	}
}

func TestFormatValue_Interface(t *testing.T) {
	var iface any = "interface value"
	result := formatValue(reflect.ValueOf(&iface).Elem())
	if result != "interface value" {
		t.Errorf("formatValue(interface) = %q, want %q", result, "interface value")
	}

	var nilIface any
	result = formatValue(reflect.ValueOf(&nilIface).Elem())
	if result != "" {
		t.Errorf("formatValue(nil interface) = %q, want empty", result)
	}
}

func TestFormatValue_Invalid(t *testing.T) {
	var invalid reflect.Value
	result := formatValue(invalid)
	if result != "" {
		t.Errorf("formatValue(invalid) = %q, want empty", result)
	}
}

func TestToSnakeCase(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"Name", "Name"},
		{"UserName", "User_Name"},
		{"HTTPServer", "H_T_T_P_Server"},
		{"already_snake", "already_snake"},
	}

	for _, tc := range testCases {
		result := toSnakeCase(tc.input)
		if result != tc.expected {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

type skipFieldStruct struct {
	Name   string `json:"name"`
	Secret string `json:"-"`              // json:"-" doesn't affect table output
	Skip   string `json:"skip" table:"-"` // table:"-" skips the field
}

func TestTableFormatter_Format_SkipFields(t *testing.T) {
	data := []skipFieldStruct{
		{Name: "visible", Secret: "secret-data", Skip: "also hidden"},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	// table:"-" should skip the field
	if strings.Contains(output, "SKIP") {
		t.Error("Format() should skip table:\"-\" fields")
	}
	if !strings.Contains(output, "visible") {
		t.Error("Format() missing visible field data")
	}
	// json:"-" doesn't skip from table output, field still appears with original name
	if !strings.Contains(output, "SECRET") {
		t.Error("Format() json:\"-\" should not affect table output (field should appear)")
	}
}

type unexportedStruct struct {
	Public  string
	private string //nolint:unused
}

func TestTableFormatter_Format_UnexportedFields(t *testing.T) {
	data := []unexportedStruct{
		{Public: "visible"},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "PUBLIC") {
		t.Error("Format() missing public field")
	}
	// Unexported fields should not appear
	if strings.Contains(output, "private") {
		t.Error("Format() should not include unexported fields")
	}
}

func TestTableFormatter_Format_FallbackToJSON(t *testing.T) {
	// Complex nested type that can't be easily tabularized
	data := make(chan int)

	var buf bytes.Buffer
	f := &TableFormatter{}

	// Should not panic, may fallback to JSON
	err := f.Format(&buf, data)
	// Some types may produce an error or empty output
	if err != nil {
		// Acceptable - some types can't be formatted
		t.Logf("Format(chan) error = %v (expected for unsupported type)", err)
	}
}

type sliceMapStruct struct {
	Items []string       `json:"items"`
	Meta  map[string]int `json:"meta"`
}

func TestTableFormatter_Format_NestedTypes(t *testing.T) {
	data := []sliceMapStruct{
		{Items: []string{"a", "b"}, Meta: map[string]int{"x": 1}},
	}

	var buf bytes.Buffer
	f := &TableFormatter{}

	err := f.Format(&buf, data)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()
	// Nested types should show item counts
	if !strings.Contains(output, "[2 items]") {
		t.Error("Format() should show slice item count")
	}
	if !strings.Contains(output, "{1 keys}") {
		t.Error("Format() should show map key count")
	}
}
