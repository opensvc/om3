package ini

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// probeLimit caps the lookahead used to check that a key line does not swallow
// the lines written after it.
const probeLimit = 4096

// WriteTo writes the document.
//
// The elements that were not modified since the parsing are written back
// verbatim, so writing a document nobody changed reproduces its source bytes
// exactly. The added and modified ones are encoded following f.Format.
func (f *File) WriteTo(w io.Writer) (int64, error) {
	b, err := f.Bytes()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	return int64(n), err
}

// Bytes returns the document encoding.
func (f *File) Bytes() ([]byte, error) {
	// A value is continued by the lines that follow it when they start with a
	// space, a tab or a form feed, so what a key line means depends on what
	// comes after it. Render the nodes backwards, each one knowing the lines
	// it is about to be followed by.
	bodies := make([][]byte, len(f.nodes))
	probe := leadingContinuationLines(f.epilogue)
	for i := len(f.nodes) - 1; i >= 0; i-- {
		n := f.nodes[i]
		body, err := f.renderNode(i, n, probe)
		if err != nil {
			return nil, err
		}
		bodies[i] = body
		probe = leadingContinuationLines(n.trivia, body, probe)
	}

	buf := bytes.NewBuffer(nil)
	buf.Write(f.bom)
	for i, n := range f.nodes {
		// The byte order mark is not content: a section header written right
		// after it still heads the document.
		if f.separatesSection(n, bodies[i], buf.Bytes()[len(f.bom):]) {
			buf.WriteString(f.Format.LineBreak)
		}
		buf.Write(n.trivia)
		buf.Write(bodies[i])
	}
	buf.Write(f.epilogue)

	out := buf.Bytes()
	if bytes.HasPrefix(out, bomUTF16BE) || bytes.HasPrefix(out, bomUTF16LE) {
		// Reading this back would be refused as a UTF-16 document. It takes a
		// key name starting with those two bytes to get here.
		return nil, fmt.Errorf("the document would start with a UTF-16 byte order mark")
	}
	return out, nil
}

// separatesSection reports whether a blank line is to be written before the
// node, which is the case of an encoded section header that neither heads the
// document nor already has a blank line before it.
//
// A section header written back from its source bytes is left alone, so a
// document nobody reformatted is reproduced exactly. Written is what has been
// written so far, the trivia of the node excluded: the blank line goes before
// the comments heading the section, which document it.
func (f *File) separatesSection(n *node, body, written []byte) bool {
	if !f.Format.BlankLineBeforeSection {
		return false
	}
	if n.section == nil || n.body != nil || len(body) == 0 {
		return false
	}
	if len(written) == 0 {
		return false
	}
	return !endsWithBlankLine(written)
}

// endsWithBlankLine reports whether b ends with an empty line, a line holding
// nothing but whitespace included.
func endsWithBlankLine(b []byte) bool {
	if !endsWithLineBreak(b) {
		return false
	}
	b = b[:len(b)-1]
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		b = b[i+1:]
	}
	return len(bytes.TrimSpace(b)) == 0
}

// renderNode returns the bytes of a node, probe being the lines it is about to
// be followed by that a value could swallow.
func (f *File) renderNode(i int, n *node, probe []byte) ([]byte, error) {
	switch {
	case n.section != nil:
		if n.body != nil && (endsWithLineBreak(n.body) || f.isLastWritten(i)) {
			return n.body, nil
		}
		if n.section.name == f.opts.DefaultSectionName && !f.Format.DefaultHeader {
			return nil, nil
		}
		return []byte("[" + n.section.name + "]" + f.Format.LineBreak), nil
	case n.key != nil:
		if n.body != nil && (endsWithLineBreak(n.body) || f.isLastWritten(i)) && f.keepsMeaning(n, probe) {
			return n.body, nil
		}
		body, err := encodeKeyLine(f.opts, f.Format, n.key, probe)
		if err != nil {
			return nil, fmt.Errorf("section %q: key %q: %w", n.key.s.name, n.key.name, err)
		}
		return []byte(body), nil
	}
	return nil, nil
}

// keepsMeaning reports whether the source text of a key still parses to the
// value it was parsed from once the lines that now follow it are appended.
func (f *File) keepsMeaning(n *node, probe []byte) bool {
	if len(probe) == 0 {
		return true
	}
	return parsesToKey(f.opts, n.body, probe, n.key.name, n.key.value)
}

// probeSectionName heads the throwaway document a candidate key line is parsed
// in to check it is read back as the key it was rendered from.
const probeSectionName = "\x00ini-probe"

// parsesToKey reports whether body, followed by probe, is read as a section
// holding the named key with the given value.
//
// The check is run on a whole document rather than on the key line alone,
// because a line is not read for itself: one starting with '#', ';' or '[' is
// not a key line at all, and one starting with a space continues the value
// above it.
func parsesToKey(opts Options, body, probe []byte, name, value string) bool {
	doc := make([]byte, 0, len(body)+len(probe)+len(probeSectionName)+3)
	doc = append(doc, '[')
	doc = append(doc, probeSectionName...)
	doc = append(doc, ']', '\n')
	doc = append(doc, body...)
	doc = append(doc, probe...)

	f, err := Load(opts, doc)
	if err != nil {
		return false
	}
	s, err := f.GetSection(probeSectionName)
	if err != nil {
		return false
	}
	k, err := s.GetKey(name)
	return err == nil && k.Value() == value
}

// isLastWritten reports whether the node of rank i is the last thing written.
func (f *File) isLastWritten(i int) bool {
	return i == len(f.nodes)-1 && len(f.epilogue) == 0
}

func endsWithLineBreak(b []byte) bool {
	return len(b) > 0 && b[len(b)-1] == '\n'
}

// leadingContinuationLines returns the complete lines heading the given parts
// that a value would be continued by, which are the ones starting with a
// space, a tab or a form feed.
func leadingContinuationLines(parts ...[]byte) []byte {
	out := make([]byte, 0, 64)
	for _, part := range parts {
		for len(part) > 0 {
			if !isContinuationIndent(part[0]) {
				return out
			}
			i := bytes.IndexByte(part, '\n')
			if i < 0 {
				out = append(out, part...)
				if len(out) > probeLimit {
					return out[:probeLimit]
				}
				break
			}
			out = append(out, part[:i+1]...)
			if len(out) > probeLimit {
				return out[:probeLimit]
			}
			part = part[i+1:]
		}
	}
	return out
}

// String returns the document encoding, empty when it can not be encoded.
func (f *File) String() string {
	b, err := f.Bytes()
	if err != nil {
		return ""
	}
	return string(b)
}

// encodeKeyLine renders a key line that parses back to the given name and
// value, probe being the lines the key line is about to be followed by.
//
// The candidate renderings are tried from the most readable to the most
// quoted, and each is parsed back, the probe appended, before being accepted.
// A value is therefore never written in a form the parser would read
// differently, whatever follows it.
func encodeKeyLine(opts Options, format Format, k *Key, probe []byte) (string, error) {
	name, value := k.name, k.value
	nameCandidates := quoteCandidates(name)
	if k.autoIncrement {
		// An auto-incremented name is not writable as such: "#1" would be
		// read back as a comment. Write the marker the number came from,
		// which is numbered "#1" again in the one section document the
		// candidate is checked in.
		name = "#1"
		nameCandidates = []string{"-"}
	}
	for _, encodedName := range nameCandidates {
		for _, encodedValue := range quoteCandidates(value) {
			body := encodedName + format.KeyValueDelimiter + encodedValue + format.LineBreak
			if parsesToKey(opts, []byte(body), probe, name, value) {
				return body, nil
			}
		}
	}
	return "", fmt.Errorf("no quoting of this value parses back to it: %q", value)
}

// quoteCandidates lists the renderings of a name or a value, from the least to
// the most quoted.
//
// The bare rendering comes first, so a value continued on the following lines
// keeps the indented layout the OpenSVC configuration files use rather than
// being wrapped in triple quotes.
func quoteCandidates(s string) []string {
	l := []string{s}
	if !strings.Contains(s, `"`) {
		l = append(l, `"`+s+`"`)
	}
	if !strings.Contains(s, "`") {
		l = append(l, "`"+s+"`")
	}
	if !strings.Contains(s, `"""`) {
		l = append(l, `"""`+s+`"""`)
	}
	return l
}
