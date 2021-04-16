package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/golang-collections/collections/set"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"gopkg.in/ini.v1"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/xstrings"
)

type (
	// T exposes methods to read and write configurations.
	T struct {
		ConfigFilePath string
		Path           path.T
		Referrer       Referrer
		file           *ini.File
	}

	// Referer is the interface implemented by node and object to
	// provide a reference resolver using their private attributes.
	Referrer interface {
		KeywordLookup(key.T) keywords.Keyword
		Dereference(string) string
		PostCommit() error
		IsVolatile() bool
	}

	Raw map[string]map[string]string

	// Op is the int representation of an operation on a key value
	Op int
)

const (
	OpUnknown Op = iota
	OpSet
	OpAppend
	OpRemove
	OpMerge
)

var (
	RegexpReference = regexp.MustCompile(`({[\w.-_:]+})`)
	RegexpOperation = regexp.MustCompile(`(\$\(\(.+\)\))`)
	RegexpScope     = regexp.MustCompile(`(@[\w.-_]+)`)
	ErrorExists     = errors.New("configuration does not exist")
	ErrNoKeyword    = errors.New("keyword does not exist")
)

func (t *T) Get(k key.T) string {
	val := t.file.Section(k.Section).Key(k.Option).Value()
	return val
}

func (t *T) valueAndKeyword(k key.T) (string, keywords.Keyword) {
	val := t.file.Section(k.Section).Key(k.Option).Value()
	kw := t.Referrer.KeywordLookup(k)
	log.Debug().Msgf("config %s get %s => %s", t.ConfigFilePath, k, val)
	return val, kw
}

func (t *T) GetString(k key.T) string {
	val, kw := t.valueAndKeyword(k)
	switch {
	case kw.IsZero():
		return ""
	case val == "" && kw.Default != "":
		return kw.Default
	}
	return val
}

func (t *T) GetStringStrict(k key.T) (string, error) {
	val, kw := t.valueAndKeyword(k)
	if kw.IsZero() {
		return "", ErrNoKeyword
	}
	return val, nil
}

func (t *T) GetSlice(k key.T) []string {
	val, _ := t.GetSliceStrict(k)
	return val
}

func (t *T) GetSliceStrict(k key.T) ([]string, error) {
	val, kw := t.valueAndKeyword(k)
	if kw.IsZero() {
		return []string{}, ErrNoKeyword
	}
	return kw.Converter.ToSlice(val)
}

func (t *T) Set(k key.T, op Op, val interface{}) error {
	switch op {
	case OpSet:
		t.file.Section(k.Section).Key(k.Option).SetValue(val.(string))
	default:
		return fmt.Errorf("unsupported operator: %d", op)
	}
	return nil
}

func (t *T) write() (err error) {
	var f *os.File
	ini.DefaultHeader = true
	dir := filepath.Dir(t.ConfigFilePath)
	base := filepath.Base(t.ConfigFilePath)
	if f, err = ioutil.TempFile(dir, "."+base+".*"); err != nil {
		return err
	}
	fName := f.Name()
	defer os.Remove(fName)
	if err = t.file.SaveTo(fName); err != nil {
		return err
	}
	if _, err = t.file.WriteTo(f); err != nil {
		return err
	}
	return os.Rename(fName, t.ConfigFilePath)
}

//
// Get returns a key value,
// * contextualized for a node (by default the local node, customized by the
//   impersonate option)
// * dereferenced
// * evaluated
//
func (t *T) Eval(k key.T) (interface{}, error) {
	var (
		err error
		ok  bool
	)
	v, err := t.descope(k)
	if err != nil {
		return nil, err
	}
	var sv string
	if sv, ok = v.(string); !ok {
		return v, nil
	}
	sv = RegexpReference.ReplaceAllStringFunc(sv, func(ref string) string {
		return t.dereference(ref, k.Section)
	})
	return sv, err
}

func (t T) sectionMap(section string) (map[string]string, error) {
	s, err := t.file.GetSection(section)
	if err != nil {
		return nil, errors.Wrapf(ErrorExists, "section '%s'", section)
	}
	return s.KeysHash(), nil
}

func (t *T) descope(k key.T) (interface{}, error) {
	s, err := t.sectionMap(k.Section)
	if err != nil {
		return nil, err
	}
	if v, ok := s[k.Option+"@"+Node.Hostname]; ok {
		return v, nil
	}
	if v, ok := s[k.Option+"@nodes"]; ok && t.IsInNodes() {
		return v, nil
	}
	if v, ok := s[k.Option+"@drpnodes"]; ok && t.IsInDRPNodes() {
		return v, nil
	}
	if v, ok := s[k.Option+"@encapnodes"]; ok && t.IsInEncapNodes() {
		return v, nil
	}
	if v, ok := s[k.Option]; ok {
		return v, nil
	}
	return nil, errors.Wrapf(ErrorExists, "key '%s' not found (tried scopes too)", k)
}

func (t T) Raw() Raw {
	data := make(Raw)
	for _, s := range t.file.Sections() {
		data[s.Name()] = s.KeysHash()
	}
	return data
}

func (t T) SectionStrings() []string {
	return t.file.SectionStrings()
}

func (t *T) Nodes() []string {
	v := t.Get(key.Parse("nodes"))
	l := strings.Fields(v)
	if len(l) == 0 && os.Getenv("OSVC_CONTEXT") == "" {
		return []string{Node.Hostname}
	}
	return t.ExpandNodes(l)
}

func (t *T) DRPNodes() []string {
	v := t.Get(key.Parse("drpnodes"))
	l := strings.Fields(v)
	return t.ExpandNodes(l)
}

func (t *T) EncapNodes() []string {
	v := t.Get(key.Parse("encapnodes"))
	l := strings.Fields(v)
	return t.ExpandNodes(l)
}

func (t *T) ExpandNodes(nodes []string) []string {
	l := make([]string, 0)
	for _, n := range nodes {
		if strings.ContainsAny(n, "=") {
			l = append(l, t.NodesWithLabel(n)...)
		} else {
			l = append(l, n)
		}
	}
	return l
}

func (t *T) NodesWithLabel(label string) []string {
	l := make([]string, 0)
	/*
		e := strings.Split(label, "=")
		n := e[0]
		v := e[1]
	*/
	// TODO iterate nodes labels
	return l
}

func (t *T) IsInNodes() bool {
	s := set.New()
	for _, n := range t.Nodes() {
		s.Insert(n)
	}
	return s.Has(Node.Hostname)
}

func (t *T) IsInDRPNodes() bool {
	s := set.New()
	for _, n := range t.DRPNodes() {
		s.Insert(n)
	}
	return s.Has(Node.Hostname)
}

func (t *T) IsInEncapNodes() bool {
	s := set.New()
	for _, n := range t.EncapNodes() {
		s.Insert(n)
	}
	return s.Has(Node.Hostname)
}

func (t T) dereference(ref string, section string) string {
	val := ""
	ref = ref[1 : len(ref)-1]
	l := strings.SplitN(ref, ":", 2)
	switch l[0] {
	case "upper":
		val = t.dereferenceWellKnown(l[1], section)
		val = strings.ToUpper(val)
	case "lower":
		val = t.dereferenceWellKnown(l[1], section)
		val = strings.ToLower(val)
	case "capitalize":
		val = t.dereferenceWellKnown(l[1], section)
		val = xstrings.Capitalize(val)
	case "title":
		val = t.dereferenceWellKnown(l[1], section)
		val = strings.Title(val)
	case "swapcase":
		val = t.dereferenceWellKnown(l[1], section)
		val = xstrings.SwapCase(val)
	default:
		val = t.dereferenceWellKnown(ref, section)
	}
	return val
}

func (t T) dereferenceWellKnown(ref string, section string) string {
	switch ref {
	case "nodename":
		return Node.Hostname
	case "short_nodename":
		return strings.SplitN(Node.Hostname, ".", 1)[0]
	case "rid":
		return section
	case "rindex":
		l := strings.SplitN(section, "#", 2)
		if len(l) != 2 {
			return section
		}
		return l[1]
	case "svcmgr":
		return os.Args[0] + " svc"
	case "nodemgr":
		return os.Args[0] + " node"
	case "etc":
		return Node.Paths.Etc
	case "var":
		return Node.Paths.Var
	default:
		if t.Referrer != nil {
			return t.Referrer.Dereference(ref)
		}
	}
	return ref
}

func (t Raw) Render() string {
	s := ""
	for section, data := range t {
		if s == "metadata" {
			continue
		}
		s += Node.Colorize.Primary(fmt.Sprintf("[%s]\n", section))
		for k, v := range data {
			if k == "comment" {
				s += renderComment(k, v)
				continue
			}
			s += renderKey(k, v)
		}
		s += "\n"
	}
	return s
}

func renderComment(k string, v interface{}) string {
	vs, ok := v.(string)
	if !ok {
		return ""
	}
	return "# " + strings.ReplaceAll(vs, "\n", "\n# ") + "\n"
}

func renderKey(k string, v interface{}) string {
	k = RegexpScope.ReplaceAllString(k, Node.Colorize.Error("$1"))
	vs, ok := v.(string)
	if ok {
		vs = RegexpReference.ReplaceAllString(vs, Node.Colorize.Optimal("$1"))
		vs = strings.ReplaceAll(vs, "\n", "\n\t")
	} else {
		vs = ""
	}
	return fmt.Sprintf("%s = %s\n", Node.Colorize.Secondary(k), vs)
}

func (t T) replaceFile(configData Raw) {
	file := ini.Empty()
	for section, m := range configData {
		for option, value := range m {
			file.Section(section).Key(option).SetValue(value)
		}
	}
	t.file = file
}

func (t T) deleteSection(section string) {
	if _, err := t.file.GetSection(section); err != nil {
		return
	}
	t.file.DeleteSection(section)
}

func (t T) initDefaultSection() error {
	defaultSection, err := t.file.GetSection("DEFAULT")
	if err != nil {
		defaultSection, err = t.file.NewSection("DEFAULT")
		if err != nil {
			return err
		}
	}
	if !defaultSection.HasKey("id") {
		_, err = defaultSection.NewKey("id", uuid.New().String())
		if err != nil {
			return err
		}
	}
	return nil
}

func (t T) rawCommit(configData Raw, configPath string, validate bool) error {
	if configData != nil {
		t.replaceFile(configData)
	}
	if configPath == "" {
		configPath = t.ConfigFilePath
	}
	if len(t.file.Sections()) == 0 {
		return nil
	}
	t.deleteSection("metadata")
	if err := t.initDefaultSection(); err != nil {
		return err
	}
	if validate {
		if err := t.validate(); err != nil {
			return err
		}
	}
	if !t.Referrer.IsVolatile() {
		if err := t.write(); err != nil {
			return err
		}
	}
	//t.clearRefCache()
	return t.postCommit()
}

func (t T) validate() error {
	return nil
}

func (t T) Commit() error {
	return t.rawCommit(nil, "", true)
}

func (t T) CommitInvalid() error {
	return t.rawCommit(nil, "", false)
}

func (t T) CommitTo(configPath string) error {
	return t.rawCommit(nil, configPath, true)
}

func (t T) CommitToInvalid(configPath string) error {
	return t.rawCommit(nil, configPath, false)
}

func (t T) CommitDataTo(configData Raw, configPath string) error {
	return t.rawCommit(configData, configPath, true)
}

func (t T) CommitDataToInvalid(configData Raw, configPath string) error {
	return t.rawCommit(configData, configPath, false)
}

func (t T) postCommit() error {
	return nil
}

func (t T) DeleteSections(sections []string) error {
	deleted := 0
	for _, section := range sections {
		if _, err := t.file.GetSection(section); err != nil {
			continue
		}
		t.file.DeleteSection(section)
		deleted++
	}
	if deleted > 0 {
		t.Commit()
	}
	return nil
}
