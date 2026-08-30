// Package ini implements a lossless reader and writer for the INI dialect used
// by the OpenSVC configuration files.
//
// The parse tree keeps a reference to the source bytes each element was
// produced from. Elements left untouched are written back verbatim, so writing
// a document nobody modified reproduces the input byte for byte, comments,
// blank lines, alignment and multiline layout included. Only the elements
// actually modified are re-encoded.
//
// This is what lets "config edit" round trip a configuration file through the
// daemon without reformatting it, and lets a value be replaced in place, as
// "config show --redact-secrets" needs.
package ini

import "io"

const (
	// DefaultSection is the name of the section holding the keys defined
	// before the first section header.
	DefaultSection = "DEFAULT"

	// defaultKeyValueDelimiters lists the runes accepted as key/value
	// separator on read.
	defaultKeyValueDelimiters = "=:"

	// defaultChildSectionDelimiter separates a child section name from its
	// parent name.
	defaultChildSectionDelimiter = "."
)

// bomUTF8 is the byte order mark stripped from the head of a source and
// written back unchanged.
var bomUTF8 = []byte{0xEF, 0xBB, 0xBF}

// bomUTF16BE and bomUTF16LE head the documents this package refuses.
var (
	bomUTF16BE = []byte{0xFE, 0xFF}
	bomUTF16LE = []byte{0xFF, 0xFE}
)

type (
	// Options tunes the accepted syntax.
	Options struct {
		// Loose does not error out when a source file does not exist.
		Loose bool

		// AllowPythonMultilineValues continues a value on the following
		// lines when those lines begin with a space, a tab or a form feed.
		AllowPythonMultilineValues bool

		// SpaceBeforeInlineComment requires a whitespace before the '#' or
		// ';' introducing an inline comment. Without it, any '#' or ';' in a
		// value starts a comment.
		SpaceBeforeInlineComment bool

		// IgnoreInlineComment keeps the '#' and ';' markers and the text
		// following them in the value.
		IgnoreInlineComment bool

		// IgnoreContinuation does not join a line ending with a backslash
		// with the line that follows.
		IgnoreContinuation bool

		// KeyValueDelimiters lists the runes accepted as key/value separator
		// on read. Defaults to "=:".
		KeyValueDelimiters string

		// ChildSectionDelimiter separates a child section name from its
		// parent name. A key missing from a child section is looked up in
		// its parent. Defaults to ".".
		ChildSectionDelimiter string

		// DefaultSectionName is the name of the section holding the keys
		// defined before the first section header. Defaults to "DEFAULT".
		DefaultSectionName string
	}

	// Format tunes the encoding of the elements that have no source bytes to
	// be written back from: the ones added or modified since the parsing.
	//
	// It has no effect on the untouched elements, which are always written
	// back verbatim.
	Format struct {
		// KeyValueDelimiter is written between a key name and its value,
		// surrounding spaces included. Defaults to " = ".
		KeyValueDelimiter string

		// LineBreak terminates a written line. Defaults to "\n".
		LineBreak string

		// DefaultHeader writes the default section header of a document
		// created empty. A parsed document is written back with the header
		// it had, or without one if it had none.
		DefaultHeader bool
	}

	// Writer is the interface WriteTo writes to.
	Writer = io.Writer
)

func (o *Options) withDefaults() {
	if o.KeyValueDelimiters == "" {
		o.KeyValueDelimiters = defaultKeyValueDelimiters
	}
	if o.ChildSectionDelimiter == "" {
		o.ChildSectionDelimiter = defaultChildSectionDelimiter
	}
	if o.DefaultSectionName == "" {
		o.DefaultSectionName = DefaultSection
	}
}

func (f *Format) withDefaults() {
	if f.KeyValueDelimiter == "" {
		f.KeyValueDelimiter = " = "
	}
	if f.LineBreak == "" {
		f.LineBreak = "\n"
	}
}
