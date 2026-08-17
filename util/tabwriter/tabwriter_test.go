package tabwriter

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func TestWriter_Basic(t *testing.T) {
	var builder strings.Builder
	w := NewWriter(&builder, 1, 1, 1, ' ', 0)

	fmt.Fprintln(w, "col1\tcol2\tcol3")
	fmt.Fprintln(w, "line1-col1\tline1-col2\tline1-col3")
	fmt.Fprintln(w, "line2-col1\tline2-col2\tline2-col3")
	w.Flush()

	output := builder.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check we have 3 lines
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// Check first line (header)
	if !strings.Contains(lines[0], "col1") || !strings.Contains(lines[0], "col2") || !strings.Contains(lines[0], "col3") {
		t.Errorf("Header line doesn't contain expected columns: %s", lines[0])
	}
}

func TestWriter_WithColors(t *testing.T) {
	// Disable color output for testing
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var builder strings.Builder
	w := NewWriter(&builder, 1, 1, 1, ' ', 0)

	green := color.New(color.FgGreen).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()

	fmt.Fprintln(w, green("col1")+"\t"+blue("col2")+"\tcol3")
	fmt.Fprintln(w, "line1-col1\tline1-col2\tline1-col3")
	w.Flush()

	output := builder.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Check we have 2 lines
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}

	// With colors disabled, check content is present
	if !strings.Contains(output, "col1") || !strings.Contains(output, "col2") || !strings.Contains(output, "line1-col1") {
		t.Errorf("Output doesn't contain expected content: %s", output)
	}
}

func TestWriter_CellAPI(t *testing.T) {
	// Disable color output for testing
	color.NoColor = true
	defer func() { color.NoColor = false }()

	var builder strings.Builder
	w := NewWriter(&builder, 1, 1, 1, ' ', 0)

	w.WriteColored(
		NewCell("Name", color.FgGreen),
		NewCell("Status", color.FgBlue),
		NewCell("Count", color.FgRed),
	)
	w.NewLine()
	w.WriteColored(
		NewCell("Server1"),
		NewCell("Running"),
		NewCell("100"),
	)
	w.Flush()

	output := builder.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d: %v", len(lines), lines)
	}

	// Check content
	if !strings.Contains(output, "Name") || !strings.Contains(output, "Status") || !strings.Contains(output, "Server1") {
		t.Errorf("Output doesn't contain expected content: %s", output)
	}
}

func TestVisibleWidth(t *testing.T) {
	// Test with no ANSI codes
	if visibleWidth("hello") != 5 {
		t.Errorf("Expected visibleWidth('hello') = 5, got %d", visibleWidth("hello"))
	}

	// Test with ANSI codes (color codes are typically like \x1b[32m)
	colored := "\x1b[32mhello\x1b[0m"
	if visibleWidth(colored) != 5 {
		t.Errorf("Expected visibleWidth(colored string) = 5, got %d", visibleWidth(colored))
	}

	// Test with multiple ANSI codes
	multi := "\x1b[1m\x1b[32mhello\x1b[0m\x1b[31mworld\x1b[0m"
	if visibleWidth(multi) != 10 {
		t.Errorf("Expected visibleWidth(multi-colored string) = 10, got %d", visibleWidth(multi))
	}
}

func TestStripANSI(t *testing.T) {
	// Test with no ANSI codes
	if stripANSI("hello") != "hello" {
		t.Errorf("Expected stripANSI('hello') = 'hello', got '%s'", stripANSI("hello"))
	}

	// Test with ANSI codes
	colored := "\x1b[32mhello\x1b[0m"
	if stripANSI(colored) != "hello" {
		t.Errorf("Expected stripANSI(colored) = 'hello', got '%s'", stripANSI(colored))
	}

	// Test with text before and after
	mixed := "prefix\x1b[32mhello\x1b[0msuffix"
	if stripANSI(mixed) != "prefixhellosuffix" {
		t.Errorf("Expected stripANSI(mixed) = 'prefixhellosuffix', got '%s'", stripANSI(mixed))
	}
}

func TestCell_String(t *testing.T) {
	// Test cell without color
	cell := NewCell("hello")
	if cell.String() != "hello" {
		t.Errorf("Expected cell.String() = 'hello', got '%s'", cell.String())
	}

	// Test cell with color (disabled for testing)
	color.NoColor = true
	defer func() { color.NoColor = false }()

	cell2 := NewCell("hello", color.FgGreen)
	if cell2.String() != "hello" {
		t.Errorf("Expected cell2.String() = 'hello' (no color), got '%s'", cell2.String())
	}
}
