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
	"github.com/iancoleman/orderedmap"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/ini.v1"
	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/converters"
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
		Log() *zerolog.Logger
	}
)

var (
	RegexpReference = regexp.MustCompile(`({[\w.-_:]+})`)
	RegexpOperation = regexp.MustCompile(`(\$\(\(.+\)\))`)
	RegexpScope     = regexp.MustCompile(`(@[\w.-_]+)`)
	ErrorExists     = errors.New("configuration does not exist")
	ErrNoKeyword    = errors.New("keyword does not exist")

	DriverGroups = set.New("ip", "volume", "disk", "fs", "share", "container", "app", "sync", "task")
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
		t.Referrer.Log().Debug().Stringer("key", k).Msg("keyword not found")
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
	return converters.ToList(val)
}

func (t *T) GetBool(k key.T) bool {
	val, _ := t.GetBoolStrict(k)
	return val
}

func (t *T) GetBoolStrict(k key.T) (bool, error) {
	val, kw := t.valueAndKeyword(k)
	if kw.IsZero() {
		return false, ErrNoKeyword
	}
	return converters.ToBool(val)
}

// Unset deletes keys and returns the number of deleted keys
func (t *T) Unset(ks ...key.T) int {
	deleted := 0
	for _, k := range ks {
		if !t.file.Section(k.Section).HasKey(k.Option) {
			continue
		}
		t.file.Section(k.Section).DeleteKey(k.Option)
		deleted += 1
	}
	return deleted
}

func (t *T) Set(op keyop.T) error {
	if !DriverGroups.Has(op.Key.Section) {
		return t.set(op)
	}
	return t.DriverGroupSet(op)
}

func (t *T) DriverGroupSet(op keyop.T) error {
	prefix := op.Key.Section + "#"
	for _, section := range t.file.SectionStrings() {
		if !strings.HasPrefix(section, prefix) {
			continue
		}
		op.Key.Section = section
		if err := t.set(op); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) set(op keyop.T) error {
	t.Referrer.Log().Debug().Stringer("op", op).Msg("set")
	setSet := func(op keyop.T) error {
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(op.Value)
		return nil
	}
	setAppend := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option).Value()
		target := ""
		if current == "" {
			target = op.Value
		} else {
			target = fmt.Sprintf("%s %s", current, op.Value)
		}
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(target)
		return nil
	}
	setMerge := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		currentSet := set.New()
		for _, e := range current {
			currentSet.Insert(e)
		}
		if currentSet.Has(op.Value) {
			return nil
		}
		return setAppend(op)
	}

	setRemove := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		target := []string{}
		removed := 0
		for _, e := range current {
			if e == op.Value {
				removed += 1
				continue
			}
			target = append(target, e)
		}
		if removed == 0 {
			return nil
		}
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(strings.Join(target, " "))
		return nil
	}

	setToggle := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		hasValue := false
		for _, e := range current {
			if e == op.Value {
				hasValue = true
				break
			}
		}
		if hasValue {
			return setRemove(op)
		}
		return setMerge(op)
	}

	setInsert := func(op keyop.T) error {
		current := strings.Fields(t.file.Section(op.Key.Section).Key(op.Key.Option).Value())
		target := []string{}
		target = append(target, current[:op.Index]...)
		target = append(target, op.Value)
		target = append(target, current[op.Index:]...)
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(strings.Join(target, " "))
		return nil
	}

	switch op.Op {
	case keyop.Set:
		return setSet(op)
	case keyop.Append:
		return setAppend(op)
	case keyop.Remove:
		return setRemove(op)
	case keyop.Merge:
		return setMerge(op)
	case keyop.Toggle:
		return setToggle(op)
	case keyop.Insert:
		return setInsert(op)
	}
	return fmt.Errorf("unsupported operator: %d", op.Op)
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
func (t *T) Eval(k key.T, impersonate string) (v string, err error) {
	v, err = t.descope(k, impersonate)
	if err != nil {
		return "", err
	}
	return t.replaceReferences(v, k.Section, impersonate), nil
}

func (t *T) replaceReferences(v string, section string, impersonate string) string {
	v = RegexpReference.ReplaceAllStringFunc(v, func(ref string) string {
		return t.dereference(ref, section, impersonate)
	})
	return v
}

func (t T) sectionMap(section string) (map[string]string, error) {
	s, err := t.file.GetSection(section)
	if err != nil {
		return nil, errors.Wrapf(ErrorExists, "section '%s'", section)
	}
	return s.KeysHash(), nil
}

func (t *T) descope(k key.T, impersonate string) (string, error) {
	if impersonate == "" {
		impersonate = Node.Hostname
	}
	s, err := t.sectionMap(k.Section)
	if err != nil {
		return "", err
	}
	if v, ok := s[k.Option+"@"+impersonate]; ok {
		return v, nil
	}
	if v, ok := s[k.Option+"@nodes"]; ok && t.IsInNodes(impersonate) {
		return v, nil
	}
	if v, ok := s[k.Option+"@drpnodes"]; ok && t.IsInDRPNodes(impersonate) {
		return v, nil
	}
	if v, ok := s[k.Option+"@encapnodes"]; ok && t.IsInEncapNodes(impersonate) {
		return v, nil
	}
	if v, ok := s[k.Option]; ok {
		return v, nil
	}
	return "", errors.Wrapf(ErrorExists, "key '%s' not found (tried scopes too)", k)
}

func (t T) Raw() Raw {
	r := Raw{}
	r.Data = orderedmap.New()
	for _, s := range t.file.Sections() {
		sectionMap := *orderedmap.New()
		for k, v := range s.KeysHash() {
			sectionMap.Set(k, v)
		}
		r.Data.Set(s.Name(), sectionMap)
	}
	return r
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

func (t *T) IsInNodes(impersonate string) bool {
	s := set.New()
	for _, n := range t.Nodes() {
		s.Insert(n)
	}
	return s.Has(impersonate)
}

func (t *T) IsInDRPNodes(impersonate string) bool {
	s := set.New()
	for _, n := range t.DRPNodes() {
		s.Insert(n)
	}
	return s.Has(impersonate)
}

func (t *T) IsInEncapNodes(impersonate string) bool {
	s := set.New()
	for _, n := range t.EncapNodes() {
		s.Insert(n)
	}
	return s.Has(impersonate)
}

func (t T) dereference(ref string, section string, impersonate string) string {
	val := ""
	ref = ref[1 : len(ref)-1]
	l := strings.SplitN(ref, ":", 2)
	switch l[0] {
	case "upper":
		val = t.dereferenceWellKnown(l[1], section, impersonate)
		val = strings.ToUpper(val)
	case "lower":
		val = t.dereferenceWellKnown(l[1], section, impersonate)
		val = strings.ToLower(val)
	case "capitalize":
		val = t.dereferenceWellKnown(l[1], section, impersonate)
		val = xstrings.Capitalize(val)
	case "title":
		val = t.dereferenceWellKnown(l[1], section, impersonate)
		val = strings.Title(val)
	case "swapcase":
		val = t.dereferenceWellKnown(l[1], section, impersonate)
		val = xstrings.SwapCase(val)
	default:
		val = t.dereferenceWellKnown(ref, section, impersonate)
	}
	return val
}

func (t T) dereferenceKey(ref string, section string, impersonate string) (v string, ok bool) {
	refKey := key.Parse(ref)
	if refKey.Section == "" {
		refKey.Section = section
	}
	key, err := t.file.Section(refKey.Section).GetKey(refKey.Option)
	if err != nil {
		return "", false
	}
	return t.replaceReferences(key.String(), refKey.Section, impersonate), true
}

func (t T) dereferenceWellKnown(ref string, section string, impersonate string) string {
	if v, ok := t.dereferenceKey(ref, section, impersonate); ok {
		return v
	}
	switch ref {
	case "nodename":
		return impersonate
	case "short_nodename":
		return strings.SplitN(impersonate, ".", 1)[0]
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
	for _, section := range configData.Data.Keys() {
		m, _ := configData.Data.Get(section)
		omap := m.(orderedmap.OrderedMap)
		for _, option := range omap.Keys() {
			value, _ := omap.Get(option)
			file.Section(section).Key(option).SetValue(value.(string))
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
	if !configData.IsZero() {
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
	return t.rawCommit(Raw{}, "", true)
}

func (t T) CommitInvalid() error {
	return t.rawCommit(Raw{}, "", false)
}

func (t T) CommitTo(configPath string) error {
	return t.rawCommit(Raw{}, configPath, true)
}

func (t T) CommitToInvalid(configPath string) error {
	return t.rawCommit(Raw{}, configPath, false)
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
