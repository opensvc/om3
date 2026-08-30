package ini

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// parser walks the source bytes of one document. It keeps the whole document in
// memory, so a value bound is a pair of offsets in data and a continued value
// is never truncated by a read buffer.
type parser struct {
	data []byte
	pos  int
	opts Options
	err  error
}

func (p *parser) eof() bool {
	return p.pos >= len(p.data)
}

// readLine consumes the next line, the trailing newline included.
func (p *parser) readLine() []byte {
	start := p.pos
	if i := bytes.IndexByte(p.data[p.pos:], '\n'); i < 0 {
		p.pos = len(p.data)
	} else {
		p.pos = start + i + 1
	}
	return p.data[start:p.pos]
}

// isContinuationIndent reports whether b can start a line continuing the value
// of the previous key.
func isContinuationIndent(b byte) bool {
	switch b {
	case ' ', '\t', '\f':
		return true
	}
	return false
}

// trimLeftSpace strips the leading whitespace, unicode.IsSpace defining it, as
// the parser this package replaces did. Whitespace is therefore not limited to
// ASCII: U+0085 and U+00A0 are stripped too.
func trimLeftSpace(b []byte) []byte {
	return bytes.TrimLeftFunc(b, unicode.IsSpace)
}

// trimRightSpaceLen returns the length of s once its trailing whitespace is
// stripped.
func trimRightSpaceLen(s string) int {
	return len(strings.TrimRightFunc(s, unicode.IsSpace))
}

// parse adds the elements of data to the document.
func (f *File) parse(data []byte) error {
	p := &parser{data: data, opts: f.opts}
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		f.bom = bomUTF8
		p.pos = len(bomUTF8)
	case bytes.HasPrefix(data, bomUTF16BE), bytes.HasPrefix(data, bomUTF16LE):
		// Refuse rather than decode: a UTF-16 document could not be written
		// back byte for byte, and OpenSVC never writes one.
		return fmt.Errorf("UTF-16 encoded configuration is not supported")
	}
	section := f.forceSection(f.opts.DefaultSectionName)
	trivia := p.pos

	// A key named "-" is numbered after its rank in the section.
	autoIncrement := 1

	for !p.eof() {
		lineStart := p.pos
		line := p.readLine()
		trimmed := trimLeftSpace(line)

		// Blank and comment lines are trivia: they are kept as the prefix of
		// the next element, so they survive a change to that element.
		if len(trimmed) == 0 || trimmed[0] == '#' || trimmed[0] == ';' {
			continue
		}
		indent := len(line) - len(trimmed)

		if trimmed[0] == '[' {
			closeIdx := bytes.LastIndexByte(trimmed, ']')
			if closeIdx < 0 {
				return fmt.Errorf("line %d: unclosed section header: %s", lineNumber(data, lineStart), bytes.TrimRight(line, "\r\n"))
			}
			name := string(trimmed[1:closeIdx])
			if name == "" {
				return fmt.Errorf("line %d: empty section name", lineNumber(data, lineStart))
			}
			section = f.forceSection(name)
			autoIncrement = 1
			f.appendNode(&node{
				trivia:  data[trivia:lineStart],
				body:    data[lineStart:p.pos],
				section: section,
			})
			trivia = p.pos
			continue
		}

		name, off, err := readKeyName(f.opts.KeyValueDelimiters, trimmed)
		if err != nil {
			return fmt.Errorf("line %d: %w", lineNumber(data, lineStart), err)
		}
		if name == "" {
			return fmt.Errorf("line %d: empty key name", lineNumber(data, lineStart))
		}
		isAutoIncrement := name == "-"
		if isAutoIncrement {
			name = "#" + strconv.Itoa(autoIncrement)
			autoIncrement++
		}
		value, valueStart, valueEnd, span := p.readValue(lineStart+indent+off, trimmed[off:])
		if p.err != nil {
			return fmt.Errorf("line %d: %w", lineNumber(data, lineStart), p.err)
		}

		if k, ok := section.index[name]; ok {
			// The key is defined again, by this document or by an earlier
			// source. The last definition wins, as it does on a reparse, and
			// the earlier line is dropped rather than kept: keeping it would
			// put a stale value back in a document a redaction just cleaned.
			//
			// The key keeps the rank of its first definition, so the document
			// order and the section order stay the same.
			k.value = value
			k.n.body = data[lineStart:p.pos]
			k.n.valueStart = valueStart - lineStart
			k.n.valueEnd = valueEnd - lineStart
			k.n.valueSpan = span
			trivia = p.pos
			continue
		}

		k := &Key{s: section, name: name, value: value, autoIncrement: isAutoIncrement}
		k.n = &node{
			trivia:     data[trivia:lineStart],
			body:       data[lineStart:p.pos],
			key:        k,
			valueStart: valueStart - lineStart,
			valueEnd:   valueEnd - lineStart,
			valueSpan:  span,
		}
		section.index[name] = k
		section.keys = append(section.keys, k)
		f.appendNode(k.n)
		trivia = p.pos
	}

	f.epilogue = data[trivia:]
	return nil
}

func lineNumber(data []byte, offset int) int {
	if offset > len(data) {
		offset = len(data)
	}
	return 1 + bytes.Count(data[:offset], []byte{'\n'})
}

// readKeyName splits a key line on its first key/value delimiter and returns
// the key name and the offset of the value in the line.
func readKeyName(delimiters string, in []byte) (string, int, error) {
	line := string(in)

	// A key name may be surrounded by quotes, to hold a delimiter.
	var keyQuote string
	if line[0] == '"' {
		if len(line) > 6 && line[0:3] == `"""` {
			keyQuote = `"""`
		} else {
			keyQuote = `"`
		}
	} else if line[0] == '`' {
		keyQuote = "`"
	}

	if len(keyQuote) > 0 {
		startIdx := len(keyQuote)
		pos := strings.Index(line[startIdx:], keyQuote)
		if pos == -1 {
			return "", -1, fmt.Errorf("missing closing key quote: %s", strings.TrimRight(line, "\r\n"))
		}
		pos += startIdx
		i := strings.IndexAny(line[pos+startIdx:], delimiters)
		if i < 0 {
			return "", -1, fmt.Errorf("key/value delimiter not found: %s", strings.TrimRight(line, "\r\n"))
		}
		return strings.TrimSpace(line[startIdx:pos]), pos + i + startIdx + 1, nil
	}

	endIdx := strings.IndexAny(line, delimiters)
	if endIdx < 0 {
		return "", -1, fmt.Errorf("key/value delimiter not found: %s", strings.TrimRight(line, "\r\n"))
	}
	if endIdx == 0 {
		return "", -1, fmt.Errorf("empty key name: %s", strings.TrimRight(line, "\r\n"))
	}
	return strings.TrimSpace(line[0:endIdx]), endIdx + 1, nil
}

// Unquote strips the surrounding quote pair a value would be stripped of when
// read from a document, so a value given on a command line and the same value
// written in a document mean the same thing.
//
// The pair is stripped only when the quote appears nowhere else in the value,
// which is the rule the parser follows.
func Unquote(s string) string {
	if hasSurroundedQuote(s, '\'') || hasSurroundedQuote(s, '"') {
		return s[1 : len(s)-1]
	}
	return s
}

// hasSurroundedQuote reports whether in starts and ends with quote, and holds
// no other occurrence of it.
func hasSurroundedQuote(in string, quote byte) bool {
	return len(in) >= 2 && in[0] == quote && in[len(in)-1] == quote &&
		strings.IndexByte(in[1:], quote) == len(in)-2
}

// readValue parses the value of a key, in being the remainder of the key line
// after the delimiter, the trailing newline included, and base its offset in
// the document.
//
// It returns the value, the bounds in the document of the source text the
// value was parsed from, and whether those bounds are usable. They are not
// when the value was assembled from backslash continuation lines, the
// backslashes making the source text and the value differ.
func (p *parser) readValue(base int, in []byte) (string, int, int, bool) {
	trimmed := trimLeftSpace(in)
	line := string(trimmed)
	start := base + len(in) - len(trimmed)

	if line == "" {
		// The whole remainder was whitespace, the line terminator included.
		// Anchor the empty value at the end of its own line rather than at
		// the start of the next one, so a substitution lands there.
		anchor := 0
		for anchor < len(in) && (in[anchor] == ' ' || in[anchor] == '\t') {
			anchor++
		}
		start = base + anchor
		if p.opts.AllowPythonMultilineValues && len(in) > 0 && in[len(in)-1] == '\n' {
			return p.readContinuedValue("", start, start)
		}
		return "", start, start, true
	}

	var valQuote string
	if len(line) > 3 && line[0:3] == `"""` {
		valQuote = `"""`
	} else if line[0] == '`' {
		valQuote = "`"
	}
	if valQuote != "" {
		startIdx := len(valQuote)
		pos := strings.LastIndex(line[startIdx:], valQuote)
		if pos == -1 {
			return p.readQuotedMultilines(line, valQuote, start+startIdx)
		}
		return line[startIdx : pos+startIdx], start + startIdx, start + startIdx + pos, true
	}

	// Whether the key line was terminated tells a value continued on the
	// following lines from a value ending the document.
	lastChar := line[len(line)-1]

	lo := 0
	hi := trimRightSpaceLen(line)

	if !p.opts.IgnoreContinuation && hi > lo && line[hi-1] == '\\' {
		val, end := p.readContinuationLines(line[lo : hi-1])
		return val, start + lo, end, false
	}

	if !p.opts.IgnoreInlineComment {
		seg := line[lo:hi]
		i := -1
		if p.opts.SpaceBeforeInlineComment {
			if i = strings.Index(seg, " #"); i == -1 {
				i = strings.Index(seg, " ;")
			}
		} else {
			i = strings.IndexAny(seg, "#;")
		}
		if i > -1 {
			hi = lo + trimRightSpaceLen(line[lo:lo+i])
		}
	}

	if seg := line[lo:hi]; hasSurroundedQuote(seg, '\'') || hasSurroundedQuote(seg, '"') {
		lo, hi = lo+1, hi-1
	} else if p.opts.AllowPythonMultilineValues && lastChar == '\n' {
		return p.readContinuedValue(line[lo:hi], start+lo, start+hi)
	}
	return line[lo:hi], start + lo, start + hi, true
}

// readContinuedValue appends to val the lines continuing it, which are the
// ones starting with a space, a tab or a form feed.
func (p *parser) readContinuedValue(val string, valueStart, valueEnd int) (string, int, int, bool) {
	for !p.eof() {
		lineStart := p.pos
		line := p.readLine()
		if len(line) == 0 || !isContinuationIndent(line[0]) {
			p.pos = lineStart
			break
		}
		content := line
		if content[len(content)-1] == '\n' {
			content = content[:len(content)-1]
		}
		val += "\n" + string(content)
		valueEnd = lineStart + len(content)
	}
	return val, valueStart, valueEnd, true
}

// readQuotedMultilines reads a value opened by a quote on the key line and
// closed on one of the following lines.
func (p *parser) readQuotedMultilines(line, valQuote string, valueStart int) (string, int, int, bool) {
	val := line[len(valQuote):]
	for {
		if p.eof() {
			p.err = fmt.Errorf("missing closing quote %s of multiline value", valQuote)
			return "", valueStart, valueStart, false
		}
		lineStart := p.pos
		next := string(p.readLine())
		if pos := strings.LastIndex(next, valQuote); pos > -1 {
			return val + next[:pos], valueStart, lineStart + pos, true
		}
		val += next
	}
}

// readContinuationLines appends to val the lines continuing it through a
// trailing backslash.
func (p *parser) readContinuationLines(val string) (string, int) {
	for !p.eof() {
		next := strings.TrimSpace(string(p.readLine()))
		if next == "" {
			break
		}
		val += next
		if val[len(val)-1] != '\\' {
			break
		}
		val = val[:len(val)-1]
	}
	return val, p.pos
}
