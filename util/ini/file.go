package ini

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type (
	// File is a parsed INI document.
	//
	// The document is held as an ordered list of nodes, one per section
	// header and one per key, each keeping the source bytes it was parsed
	// from. Sections and keys are indexes over that list.
	File struct {
		// Format tunes the encoding of the added and modified elements.
		Format Format

		opts     Options
		bom      []byte
		nodes    []*node
		sections []*Section
		index    map[string]*Section
		sources  []source
		epilogue []byte
	}

	// Section is a named group of keys.
	Section struct {
		f     *File
		name  string
		keys  []*Key
		index map[string]*Key
	}

	// Key is a name/value pair in a section.
	Key struct {
		s     *Section
		n     *node
		name  string
		value string

		// autoIncrement tells the key was written "-" and numbered after its
		// rank in the section. It is written back "-".
		autoIncrement bool
	}

	// node is a written unit: a section header or a key. It holds the bytes
	// it was parsed from, split between the trivia, which is the run of blank
	// and comment lines preceding it, and the body, which is the element
	// itself.
	//
	// A nil body means the element has no source bytes to be written back
	// from and must be encoded. The trivia survives a modification, so the
	// comments documenting a key are not lost when its value changes.
	node struct {
		trivia  []byte
		body    []byte
		section *Section
		key     *Key

		// valueStart and valueEnd delimit, in body, the source text the key
		// value was parsed from. valueSpan reports whether they are usable,
		// which backslash continuations are not.
		valueStart int
		valueEnd   int
		valueSpan  bool
	}

	// source is a document source, remembered so Reload can read it again.
	source struct {
		path string
		data []byte
	}
)

// Empty returns an empty document.
func Empty(opts ...Options) *File {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	f := newFile(o)
	// Give the default section a header to encode, so a document built from
	// scratch is written with the "[DEFAULT]" line the OpenSVC configuration
	// files are expected to carry. A parsed document keeps the header it had,
	// until MaterializeDefaultSection is called on it.
	f.MaterializeDefaultSection()
	return f
}

// Load parses the given sources into a single document.
//
// A source is either a file path, as a string, or the document bytes, as a
// []byte. Sources are merged in order: a key redefined by a later source keeps
// its first position and takes the later value.
//
// Writing back is byte-identical only when the document was loaded from a
// single source, which is the case of every document OpenSVC writes back.
func Load(opts Options, sources ...any) (*File, error) {
	f := newFile(opts)
	for _, src := range sources {
		switch data := src.(type) {
		case string:
			f.sources = append(f.sources, source{path: data})
		case []byte:
			f.sources = append(f.sources, source{data: data})
		case io.Reader:
			b, err := io.ReadAll(data)
			if err != nil {
				return nil, err
			}
			f.sources = append(f.sources, source{data: b})
		default:
			return nil, fmt.Errorf("unsupported ini source type %T", src)
		}
	}
	if err := f.load(); err != nil {
		return nil, err
	}
	return f, nil
}

func newFile(opts Options) *File {
	opts.withDefaults()
	f := &File{
		opts:  opts,
		index: make(map[string]*Section),
	}
	f.Format.DefaultHeader = true
	f.Format.BlankLineBeforeSection = true
	f.Format.withDefaults()
	return f
}

func (f *File) load() error {
	f.nodes = nil
	f.sections = nil
	f.index = make(map[string]*Section)
	f.bom = nil
	f.epilogue = nil

	// The default section always exists, even in an empty document.
	f.forceSection(f.opts.DefaultSectionName)

	for _, src := range f.sources {
		data := src.data
		if src.path != "" {
			b, err := os.ReadFile(src.path)
			switch {
			case err == nil:
				data = b
			case os.IsNotExist(err) && f.opts.Loose:
				continue
			default:
				return err
			}
		}
		if err := f.parse(data); err != nil {
			if src.path != "" {
				return fmt.Errorf("%s: %w", src.path, err)
			}
			return err
		}
	}
	return nil
}

// Reload parses the sources again, dropping every uncommitted change.
func (f *File) Reload() error {
	return f.load()
}

// Options returns the syntax options the document was parsed with.
func (f *File) Options() Options {
	return f.opts
}

func (f *File) appendNode(n *node) {
	f.nodes = append(f.nodes, n)
}

// insertNodeAfterSection inserts n right after the last node belonging to
// section s, so a key added to a section is written inside it.
func (f *File) insertNodeAfterSection(s *Section, n *node) {
	last := -1
	for i, cur := range f.nodes {
		switch {
		case cur.section == s:
			last = i
		case cur.key != nil && cur.key.s == s:
			last = i
		}
	}
	if last < 0 {
		// The section has no node yet. Only the default section of a parsed
		// document can be in that case: it always exists, even when the
		// source carries neither a "[DEFAULT]" header nor a key before the
		// first section header. Its keys are the ones written before the
		// first section header, so head the document with the node instead
		// of appending it, which would land the key in the last section.
		f.nodes = append([]*node{n}, f.nodes...)
		return
	}
	f.nodes = append(f.nodes, nil)
	copy(f.nodes[last+2:], f.nodes[last+1:])
	f.nodes[last+1] = n
}

func (f *File) removeNode(n *node) {
	for i, cur := range f.nodes {
		if cur == n {
			f.nodes = append(f.nodes[:i], f.nodes[i+1:]...)
			return
		}
	}
}

func (f *File) forceSection(name string) *Section {
	if s, ok := f.index[name]; ok {
		return s
	}
	s := &Section{
		f:     f,
		name:  name,
		index: make(map[string]*Key),
	}
	f.index[name] = s
	f.sections = append(f.sections, s)
	return s
}

// Section returns the named section, creating it when it does not exist.
func (f *File) Section(name string) *Section {
	if s, ok := f.index[name]; ok {
		return s
	}
	s, _ := f.NewSection(name)
	return s
}

// GetSection returns the named section, or an error when it does not exist.
func (f *File) GetSection(name string) (*Section, error) {
	if s, ok := f.index[name]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("section %q does not exist", name)
}

// NewSection returns the named section, creating it when it does not exist.
func (f *File) NewSection(name string) (*Section, error) {
	if name == "" {
		return nil, fmt.Errorf("empty section name")
	}
	if s, ok := f.index[name]; ok {
		return s, nil
	}
	s := f.forceSection(name)
	f.appendNode(&node{section: s})
	return s, nil
}

// MaterializeDefaultSection gives the default section a header to encode when
// it has none, so the document is written with the "[DEFAULT]" line and the
// keys of the default section are visibly grouped under it.
//
// The default section of a parsed document has no header when the source had
// none: the keys defined before the first section header are its keys, and the
// document is written back without a header it never had. Materializing the
// header inserts it at the head of the document, before those keys.
//
// A document that already has a default section header is left untouched, and
// so is its byte-identical write back. The header is encoded only when
// Format.DefaultHeader is set, which it is by default.
func (f *File) MaterializeDefaultSection() {
	s := f.forceSection(f.opts.DefaultSectionName)
	for _, n := range f.nodes {
		if n.section == s {
			return
		}
	}
	f.nodes = append([]*node{{section: s}}, f.nodes...)
}

// HasSection reports whether the named section exists.
func (f *File) HasSection(name string) bool {
	_, ok := f.index[name]
	return ok
}

// DeleteSection removes the named section and all its keys.
func (f *File) DeleteSection(name string) {
	s, ok := f.index[name]
	if !ok {
		return
	}
	kept := f.nodes[:0]
	for _, n := range f.nodes {
		if n.section == s || (n.key != nil && n.key.s == s) {
			continue
		}
		kept = append(kept, n)
	}
	f.nodes = kept
	delete(f.index, name)
	for i, cur := range f.sections {
		if cur == s {
			f.sections = append(f.sections[:i], f.sections[i+1:]...)
			break
		}
	}
}

// Sections returns the sections, in document order.
func (f *File) Sections() []*Section {
	l := make([]*Section, len(f.sections))
	copy(l, f.sections)
	return l
}

// SectionStrings returns the section names, in document order.
func (f *File) SectionStrings() []string {
	l := make([]string, len(f.sections))
	for i, s := range f.sections {
		l[i] = s.name
	}
	return l
}

// Name returns the section name.
func (s *Section) Name() string {
	return s.name
}

// Key returns the named key, creating an empty one when it does not exist.
func (s *Section) Key(name string) *Key {
	if k, err := s.GetKey(name); err == nil {
		return k
	}
	k, _ := s.NewKey(name, "")
	return k
}

// GetKey returns the named key, or an error when it does not exist.
//
// A key missing from a child section is looked up in its parent sections, the
// child section delimiter separating the names.
func (s *Section) GetKey(name string) (*Key, error) {
	if k, ok := s.index[name]; ok {
		return k, nil
	}
	sname := s.name
	for {
		i := strings.LastIndex(sname, s.f.opts.ChildSectionDelimiter)
		if i < 0 {
			break
		}
		sname = sname[:i]
		if parent, ok := s.f.index[sname]; ok {
			return parent.GetKey(name)
		}
	}
	return nil, fmt.Errorf("section %q has no key %q", s.name, name)
}

// HasKey reports whether the section, or one of its parent sections, has the
// named key.
func (s *Section) HasKey(name string) bool {
	_, err := s.GetKey(name)
	return err == nil
}

// NewKey returns the named key with its value set, creating the key when it
// does not exist.
func (s *Section) NewKey(name, value string) (*Key, error) {
	if name == "" {
		return nil, fmt.Errorf("empty key name in section %q", s.name)
	}
	if k, ok := s.index[name]; ok {
		k.SetValue(value)
		return k, nil
	}
	k := &Key{s: s, name: name, value: value}
	k.n = &node{key: k}
	s.index[name] = k
	s.keys = append(s.keys, k)
	s.f.insertNodeAfterSection(s, k.n)
	return k, nil
}

// DeleteKey removes the named key from the section.
func (s *Section) DeleteKey(name string) {
	k, ok := s.index[name]
	if !ok {
		return
	}
	delete(s.index, name)
	for i, cur := range s.keys {
		if cur == k {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			break
		}
	}
	s.f.removeNode(k.n)
}

// Keys returns the section keys, in document order.
func (s *Section) Keys() []*Key {
	l := make([]*Key, len(s.keys))
	copy(l, s.keys)
	return l
}

// KeyStrings returns the section key names, in document order.
func (s *Section) KeyStrings() []string {
	l := make([]string, len(s.keys))
	for i, k := range s.keys {
		l[i] = k.name
	}
	return l
}

// KeysHash returns the section keys as a name to value map.
func (s *Section) KeysHash() map[string]string {
	m := make(map[string]string, len(s.keys))
	for _, k := range s.keys {
		m[k.name] = k.value
	}
	return m
}

// Name returns the key name.
func (k *Key) Name() string {
	return k.name
}

// Value returns the key value.
func (k *Key) Value() string {
	return k.value
}

// String returns the key value.
func (k *Key) String() string {
	return k.value
}

// Section returns the section the key belongs to.
func (k *Key) Section() *Section {
	return k.s
}

// SetValue sets the key value and drops the source formatting of the key line,
// which is encoded again on write. The comments preceding the key are kept.
func (k *Key) SetValue(value string) {
	k.value = value
	k.n.body = nil
	k.n.valueSpan = false
}

// SetValueKeepFormat sets the key value by substituting it in the source text
// of the key line, so the key name, the spacing, the inline comment and the
// layout of a multiline value are all preserved. This is what a redaction
// needs: only the secret is replaced, the file is otherwise untouched.
//
// It falls back to SetValue when the source text is unavailable, or when
// substituting would not parse back to value, and reports whether the
// formatting could be preserved.
func (k *Key) SetValueKeepFormat(value string) bool {
	if k.n.body == nil || !k.n.valueSpan {
		k.SetValue(value)
		return false
	}
	body := make([]byte, 0, len(k.n.body)+len(value))
	body = append(body, k.n.body[:k.n.valueStart]...)
	body = append(body, value...)
	body = append(body, k.n.body[k.n.valueEnd:]...)

	// Only keep the substitution if the result is still read as this key
	// holding this value.
	if !parsesToKey(k.s.f.opts, body, nil, k.name, value) {
		k.SetValue(value)
		return false
	}
	k.n.body = body
	k.n.valueEnd = k.n.valueStart + len(value)
	k.value = value
	return true
}
