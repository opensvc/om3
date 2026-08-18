// Package tabwriter preserves ANSI color codes and aligns columns based on
// visible width (excluding ANSI escape sequences).
//
// Key features:
//   - NewWriter: API-compatible with text/tabwriter.NewWriter
//   - Cell type: supports arbitrary number of color.Attribute values
//   - WriteColored/WriteCells: convenience methods for colored tabular data
//   - Preserves ANSI codes in output while excluding them from width calculations
//   - Thread-safe implementation
package tabwriter

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
)

// stripANSI removes ANSI escape sequences from a string
func stripANSI(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '\x1b' && s[i+1] == '[' {
			// Found ANSI escape sequence, skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // skip the 'm'
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

// visibleWidth returns the visible width of a string (excluding ANSI codes)
func visibleWidth(s string) int {
	return len([]rune(stripANSI(s)))
}

// Writer is a tabwriter that preserves ANSI color codes.
// It aligns columns based on visible width (excluding ANSI escape sequences).
type Writer struct {
	writer   io.Writer
	minwidth int
	padchar  byte
	padding  int

	// Current line being built (as a buffer of bytes)
	currentLine strings.Builder
	// All complete lines (each line is a slice of cells)
	lines [][]string
	// Maximum visible width for each column
	maxWidths []int
	// Track if we've computed widths
	widthsComputed bool

	// flags specifies optional behavior configurations for the writer,
	// typically used for compatibility or future extensions.
	flags uint

	debug bool

	mu sync.Mutex
}

// NewWriter creates a new tabwriter that preserves ANSI color codes.
// Parameters match text/tabwriter.NewWriter:
//   - w: the underlying writer
//   - minwidth: minimum cell width (in visible characters, excluding ANSI codes)
//   - tabwidth: unused (kept for API compatibility)
//   - padding: padding added to each cell
//   - padchar: ASCII character used for padding
//   - flags: unused (kept for API compatibility)
//
// This writer properly handles ANSI escape sequences from github.com/fatih/color,
// aligning columns based on visible width while preserving color codes in output.
func NewWriter(w io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) *Writer {
	return &Writer{
		writer:   w,
		minwidth: minwidth,
		padchar:  padchar,
		padding:  padding,
		flags:    flags,
		debug:    flags&Debug != 0,
	}
}

// Write implements io.Writer by buffering input.
// Newlines trigger line finalization.
func (w *Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Look for newlines in the input
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '\n' {
			// Write everything up to (but not including) the newline
			w.currentLine.Write(p[start:i])
			// Finalize the current line
			lineStr := w.currentLine.String()
			if lineStr != "" {
				// Split by tabs to get cells
				cells := strings.Split(lineStr, "\t")
				w.lines = append(w.lines, cells)
			}
			w.currentLine.Reset()
			start = i + 1
		}
	}

	// Write any remaining data (without newline)
	if start < len(p) {
		w.currentLine.Write(p[start:])
	}

	return len(p), nil
}

// Cell represents a single tabular cell with color support
type Cell struct {
	content string
	attrs   []color.Attribute
}

// NewCell creates a new cell with the given content and color attributes
func NewCell(content string, attrs ...color.Attribute) Cell {
	return Cell{
		content: content,
		attrs:   attrs,
	}
}

// String returns the cell content with ANSI color codes applied
func (c Cell) String() string {
	if len(c.attrs) == 0 {
		return c.content
	}
	return color.New(c.attrs...).Sprint(c.content)
}

// WriteCells writes multiple cells (columns) as a tab-separated line with their own colors.
// Each cell is a string-content pair with color attributes.
// This is the primary method for writing colored tabular data.
func (w *Writer) WriteCells(cells []Cell) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var line strings.Builder
	for i, cell := range cells {
		if i > 0 {
			line.WriteByte('\t')
		}
		line.WriteString(cell.String())
	}
	w.currentLine.WriteString(line.String())
}

// WriteColored writes a line of cells where each cell can have its own color.
// This is a convenience method for writing a complete row with colored cells.
func (w *Writer) WriteColored(cells ...Cell) {
	w.WriteCells(cells)
}

// WriteLine writes a complete line of cells (tab-separated values).
// If cells contain ANSI codes, they are preserved and not counted for alignment.
func (w *Writer) WriteLine(cells ...string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.currentLine.WriteString(strings.Join(cells, "\t"))
}

// NewLine finishes the current line and starts a new one
func (w *Writer) NewLine() {
	w.mu.Lock()
	defer w.mu.Unlock()

	lineStr := w.currentLine.String()
	if lineStr != "" {
		// Split by tabs to get cells
		cells := strings.Split(lineStr, "\t")
		w.lines = append(w.lines, cells)
	}
	w.currentLine.Reset()
	w.widthsComputed = false
}

// Flush writes all buffered data to the underlying writer.
// This must be called to ensure all data is written.
func (w *Writer) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Finish current line if needed
	lineStr := w.currentLine.String()
	if lineStr != "" {
		cells := strings.Split(lineStr, "\t")
		w.lines = append(w.lines, cells)
		w.currentLine.Reset()
	}

	if len(w.lines) == 0 {
		return nil
	}

	// Compute max widths for each column
	w.computeWidths()

	// Write all lines
	for _, cells := range w.lines {
		if err := w.writeLine(cells); err != nil {
			return err
		}
	}

	// Reset
	w.lines = nil
	w.maxWidths = nil
	w.widthsComputed = false

	return nil
}

// computeWidths calculates the maximum visible width for each column
func (w *Writer) computeWidths() {
	maxCols := 0
	for _, cells := range w.lines {
		if len(cells) > maxCols {
			maxCols = len(cells)
		}
	}

	w.maxWidths = make([]int, maxCols)

	for col := 0; col < maxCols; col++ {
		maxWidths := 0
		for _, cells := range w.lines {
			if col < len(cells) {
				width := visibleWidth(cells[col])
				if width > maxWidths {
					maxWidths = width
				}
			}
		}

		// Add padding
		maxWidths += w.padding

		// Apply minwidth
		if maxWidths < w.minwidth {
			maxWidths = w.minwidth
		}
		w.maxWidths[col] = maxWidths
	}

	w.widthsComputed = true
}

// writeLine writes a single line with proper padding
func (w *Writer) writeLine(cells []string) error {
	for col, cell := range cells {
		// Write the cell content (with ANSI codes)
		if _, err := fmt.Fprint(w.writer, cell); err != nil {
			return err
		}

		// Calculate and write padding
		cellWidth := visibleWidth(cell)
		maxWidth := w.maxWidths[col]
		padding := maxWidth - cellWidth

		for i := 0; i < padding; i++ {
			if _, err := w.writer.Write([]byte{w.padchar}); err != nil {
				return err
			}
		}
		if w.debug {
			_, _ = fmt.Fprintf(w.writer, "|")
		}
	}

	// Write newline
	if _, err := fmt.Fprintln(w.writer); err != nil {
		return err
	}

	return nil
}

// Format helper constants for compatibility with text/tabwriter
const (
	Debug = tabwriter.Debug
)
