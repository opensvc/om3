package xconfig

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cvaroqui/ini"
	"github.com/golang-collections/collections/set"
	"github.com/google/uuid"
	"github.com/iancoleman/orderedmap"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/opensvc/om3/util/xstrings"
)

type (
	Converter func(string) (interface{}, error)

	// T exposes methods to read and write configurations.
	T struct {
		ConfigFilePath string
		Path           naming.Path
		Referrer       Referrer
		NodeReferrer   Referrer
		file           *ini.File
		postCommit     func() error
		changed        bool
	}

	// Referrer is the interface implemented by node and object to
	// provide a reference resolver using their private attributes.
	Referrer interface {
		KeywordLookup(key.T, string) keywords.Keyword
		IsVolatile() bool
		Config() *T

		// for reference private to the referrer. ex: path for an object
		Dereference(string) (string, error)

		// for scoping
		Nodes() ([]string, error)
		DRPNodes() ([]string, error)
	}

	encapNodeser interface {
		// for scoping
		EncapNodes() ([]string, error)
	}

	ErrPostponedRef struct {
		Ref string
		RID string
	}
)

var (
	RegexpOperation = regexp.MustCompile(`(\$\(\(.+\)\))`)
	ErrExist        = errors.New("configuration does not exist")
	ErrNoKeyword    = errors.New("keyword does not exist")
	ErrType         = errors.New("type error")

	DriverGroups = set.New("ip", "volume", "disk", "fs", "share", "container", "app", "sync", "task")
)

func (t ErrPostponedRef) Error() string {
	return fmt.Sprintf("ref %s evaluation postponed: resource %s is not configured", t.Ref, t.RID)
}

func NewErrPostponedRef(ref string, rid string) ErrPostponedRef {
	return ErrPostponedRef{
		Ref: ref,
		RID: rid,
	}
}

func (t T) Reload() error {
	return t.file.Reload()
}

func (t T) Changed() bool {
	return t.changed
}

// Keys returns the key names available in a section
func (t *T) Keys(section string) []string {
	data := make([]string, 0)
	for _, s := range t.file.Section(section).Keys() {
		data = append(data, s.Name())
	}
	return data
}

func (t *T) RegisterPostCommit(fn func() error) {
	t.postCommit = fn
}

// keysLike returns the slice of key.T if
// * k=app#1.type and option app#1.type exists (same as HasKey)
// -or-
// * k=app.type and option app#1.type exists
// -or-
// * k=app and section app#1 exists
func (t *T) keysLike(k key.T) []key.T {
	if k.Option == "" {
		if strings.Contains(k.Section, "#") {
			if t.HasSectionString(k.Section) {
				return []key.T{k}
			}
		}
		prefix := k.Section + "#"
		l := make([]key.T, 0)
		for _, s := range t.SectionStrings() {
			if strings.HasPrefix(s, prefix) {
				l = append(l, key.T{Section: s})
			}
		}
		return l
	}
	prefix := k.Section + "#"
	l := make([]key.T, 0)
	for _, s := range t.SectionStrings() {
		if s != k.Section && !strings.HasPrefix(s, prefix) {
			continue
		}
		if t.HasKey(key.T{Section: s, Option: k.Option}) {
			l = append(l, key.T{Section: s, Option: k.Option})
		}
	}
	return l
}

func (t *T) HasKeyMatchingOp(kop keyop.T) bool {
	compString := func(v1, v2 string, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return v1 == v2
		case op.Is(keyop.NotEqual):
			return v1 != v2
		case op.Is(keyop.Greater):
			return v1 > v2
		case op.Is(keyop.Lesser):
			return v1 < v2
		case op.Is(keyop.GreaterOrEqual):
			return v1 >= v2
		case op.Is(keyop.LesserOrEqual):
			return v1 <= v2
		}
		return false
	}
	compStringSlice := func(v1, v2 []string, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return stringslice.Equal(v1, v2)
		case op.Is(keyop.NotEqual):
			return !stringslice.Equal(v1, v2)
		}
		return false
	}
	compFloat64 := func(v1, v2 float64, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return v1 == v2
		case op.Is(keyop.NotEqual):
			return v1 != v2
		case op.Is(keyop.Greater):
			return v1 > v2
		case op.Is(keyop.Lesser):
			return v1 < v2
		case op.Is(keyop.GreaterOrEqual):
			return v1 >= v2
		case op.Is(keyop.LesserOrEqual):
			return v1 <= v2
		}
		return false
	}
	compBool := func(v1, v2 bool, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return v1 == v2
		case op.Is(keyop.NotEqual):
			return v1 != v2
		}
		return false
	}
	compInt := func(v1, v2 int, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return v1 == v2
		case op.Is(keyop.NotEqual):
			return v1 != v2
		case op.Is(keyop.Greater):
			return v1 > v2
		case op.Is(keyop.Lesser):
			return v1 < v2
		case op.Is(keyop.GreaterOrEqual):
			return v1 >= v2
		case op.Is(keyop.LesserOrEqual):
			return v1 <= v2
		}
		return false
	}
	compInt64 := func(v1, v2 int64, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return v1 == v2
		case op.Is(keyop.NotEqual):
			return v1 != v2
		case op.Is(keyop.Greater):
			return v1 > v2
		case op.Is(keyop.Lesser):
			return v1 < v2
		case op.Is(keyop.GreaterOrEqual):
			return v1 >= v2
		case op.Is(keyop.LesserOrEqual):
			return v1 <= v2
		}
		return false
	}
	compInt64Ptr := func(v1, v2 *int64, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return *v1 == *v2
		case op.Is(keyop.NotEqual):
			return *v1 != *v2
		case op.Is(keyop.Greater):
			return *v1 > *v2
		case op.Is(keyop.Lesser):
			return *v1 < *v2
		case op.Is(keyop.GreaterOrEqual):
			return *v1 >= *v2
		case op.Is(keyop.LesserOrEqual):
			return *v1 <= *v2
		}
		return false
	}
	compDurationPtr := func(v1, v2 *time.Duration, op keyop.Op) bool {
		switch {
		case op.Is(keyop.Equal):
			return *v1 == *v2
		case op.Is(keyop.NotEqual):
			return *v1 != *v2
		case op.Is(keyop.Greater):
			return *v1 > *v2
		case op.Is(keyop.Lesser):
			return *v1 < *v2
		case op.Is(keyop.GreaterOrEqual):
			return *v1 >= *v2
		case op.Is(keyop.LesserOrEqual):
			return *v1 <= *v2
		}
		return false
	}
	comp := func(v1, v2 interface{}, op keyop.Op) bool {
		switch i := v1.(type) {
		case string:
			return compString(i, v2.(string), op)
		case []string:
			return compStringSlice(i, v2.([]string), op)
		case float64:
			return compFloat64(i, v2.(float64), op)
		case bool:
			return compBool(i, v2.(bool), op)
		case int:
			return compInt(i, v2.(int), op)
		case int64:
			return compInt64(i, v2.(int64), op)
		case *int64:
			return compInt64Ptr(i, v2.(*int64), op)
		case *time.Duration:
			return compDurationPtr(i, v2.(*time.Duration), op)
		default:
			return false
		}
	}

	for _, k := range t.keysLike(kop.Key) {
		if kop.Op.Is(keyop.Exist) {
			return true
		}
		v := t.Get(k)
		sectionType := t.SectionType(k)
		kw, err := getKeyword(k, sectionType, t.Referrer)
		if err != nil {
			iv := v
			it := kop.Value
			if comp(iv, it, kop.Op) {
				return true
			}
		}
		if kw.Converter == nil {
			iv := v
			it := kop.Value
			if comp(iv, it, kop.Op) {
				return true
			}
		} else {
			iv, err := kw.Converter.Convert(v)
			if err != nil {
				continue
			}
			it, err := kw.Converter.Convert(kop.Value)
			if err != nil {
				continue
			}
			if comp(iv, it, kop.Op) {
				return true
			}
		}
	}
	return false
}

// HasKey returns true if the k exists
func (t *T) HasKey(k key.T) bool {
	if t == nil {
		return false
	}
	return t.file.Section(k.Section).HasKey(k.Option)
}

func (t *T) Get(k key.T) string {
	if section := t.file.Section(k.Section); section == nil {
		return ""
	} else if fk := section.Key(k.Option); fk == nil {
		return ""
	} else {
		return fk.Value()
	}
}

func (t *T) GetStrict(k key.T) (string, error) {
	section := t.file.Section(k.Section)
	if section.HasKey(k.Option) {
		return section.Key(k.Option).Value(), nil
	}
	return "", fmt.Errorf("%w: key '%s' not found (unscopable kw)", ErrExist, k)
}

func (t *T) GetStringAs(k key.T, impersonate string) string {
	val, _ := t.GetStringStrictAs(k, impersonate)
	return val
}

func (t *T) GetString(k key.T) string {
	val, _ := t.GetStringStrict(k)
	return val
}

func (t *T) GetStringStrictAs(k key.T, impersonate string) (string, error) {
	if v, err := t.EvalAs(k, impersonate); err != nil {
		return "", err
	} else {
		return v.(string), nil
	}
}

func (t *T) GetStringStrict(k key.T) (string, error) {
	if v, err := t.Eval(k); err != nil {
		return "", err
	} else {
		return v.(string), nil
	}
}

func (t *T) GetStrings(k key.T) []string {
	val, _ := t.GetStringsStrict(k)
	return val
}

func (t *T) GetStringsStrict(k key.T) ([]string, error) {
	if v, err := t.Eval(k); err != nil {
		return []string{}, err
	} else {
		return v.([]string), nil
	}
}

func (t *T) GetBool(k key.T) bool {
	val, _ := t.GetBoolStrict(k)
	return val
}

func (t *T) GetBoolStrict(k key.T) (bool, error) {
	if v, err := t.Eval(k); err != nil {
		return false, err
	} else if i, ok := v.(bool); !ok {
		return false, fmt.Errorf("%w: expected int, got %v", ErrType, v)
	} else {
		return i, nil
	}
}

func (t *T) GetDuration(k key.T) *time.Duration {
	val, _ := t.GetDurationStrict(k)
	return val
}

func (t *T) GetDurationStrict(k key.T) (*time.Duration, error) {
	if v, err := t.Eval(k); err != nil {
		return nil, err
	} else if i, ok := v.(*time.Duration); !ok {
		return nil, fmt.Errorf("%w: expected *time.Duration, got %v", ErrType, v)
	} else {
		return i, nil
	}
}

func (t *T) GetInt(k key.T) int {
	val, _ := t.GetIntStrict(k)
	return val
}

func (t *T) GetIntStrict(k key.T) (int, error) {
	if v, err := t.Eval(k); err != nil {
		return 0, err
	} else if i, ok := v.(int); !ok {
		return 0, fmt.Errorf("%w: expected int, got %v", ErrType, v)
	} else {
		return i, nil
	}
}

func (t *T) GetSize(k key.T) *int64 {
	val, _ := t.GetSizeStrict(k)
	return val
}

func (t *T) GetSizeStrict(k key.T) (*int64, error) {
	if v, err := t.Eval(k); err != nil {
		var i int64
		return &i, err
	} else {
		return v.(*int64), nil
	}
}

// PrepareUnset unsets keywords from config without committing changes.
func (t *T) PrepareUnset(ks ...key.T) error {
	ks, err := t.expandKeywords(ks...)
	if err != nil {
		return err
	}
	for _, k := range ks {
		if !t.file.Section(k.Section).HasKey(k.Option) {
			continue
		}
		t.file.Section(k.Section).DeleteKey(k.Option)
		t.changed = true
	}
	return nil
}

func (t *T) expandKeywords(ks ...key.T) (key.L, error) {
	var l key.L
	for _, k := range ks {
		if !DriverGroups.Has(k.Section) {
			l = append(l, k)
			continue
		}
		prefix := k.Section + "#"
		for _, section := range t.file.SectionStrings() {
			if !strings.HasPrefix(section, prefix) {
				continue
			}
			l = append(l, key.T{
				Section: section,
				Option:  k.Option,
			})
		}
	}
	return l, nil
}

// Unset deletes keys and commits.
func (t *T) Unset(ks ...key.T) error {
	if err := t.PrepareUnset(ks...); err != nil {
		return err
	}
	return t.Commit()
}

func (t *T) Set(ops ...keyop.T) error {
	if err := t.PrepareSet(ops...); err != nil {
		return err
	}
	return t.Commit()
}

func (t *T) prepareSetKey(op keyop.T) error {
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
	setSet := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option)
		if current == nil {
			return fmt.Errorf("invalid key in %s", op)
		}
		if current.Value() == op.Value {
			return nil
		}
		current.SetValue(op.Value)
		t.changed = true
		return nil
	}
	setAppend := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option)
		if current == nil {
			return fmt.Errorf("invalid key in %s", op)
		}
		target := ""
		if current.Value() == "" {
			target = op.Value
		} else {
			target = fmt.Sprintf("%s %s", current.Value(), op.Value)
		}
		current.SetValue(target)
		t.changed = true
		return nil
	}
	setMerge := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option)
		if current == nil {
			return fmt.Errorf("invalid key in %s", op)
		}
		currentFields := strings.Fields(current.Value())
		currentSet := set.New()
		for _, e := range currentFields {
			currentSet.Insert(e)
		}
		if currentSet.Has(op.Value) {
			return nil
		}
		return setAppend(op)
	}

	setRemove := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option)
		if current == nil {
			return fmt.Errorf("invalid key in %s", op)
		}
		currentFields := strings.Fields(current.Value())
		target := []string{}
		removed := 0
		for _, e := range currentFields {
			if e == op.Value {
				removed++
				continue
			}
			target = append(target, e)
		}
		if removed == 0 {
			return nil
		}
		current.SetValue(strings.Join(target, " "))
		t.changed = true
		return nil
	}

	setToggle := func(op keyop.T) error {
		current := t.file.Section(op.Key.Section).Key(op.Key.Option)
		if current == nil {
			return fmt.Errorf("invalid key in %s", op)
		}
		currentFields := strings.Fields(current.Value())
		hasValue := false
		for _, e := range currentFields {
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
		current := t.file.Section(op.Key.Section).Key(op.Key.Option)
		if current == nil {
			return fmt.Errorf("invalid key in %s", op)
		}
		currentFields := strings.Fields(current.Value())
		target := []string{}
		target = append(target, currentFields[:op.Index]...)
		target = append(target, op.Value)
		target = append(target, currentFields[op.Index:]...)
		t.file.Section(op.Key.Section).Key(op.Key.Option).SetValue(strings.Join(target, " "))
		t.changed = true
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
	return fmt.Errorf("unsupported operator: %d setting key %s", op.Op, op.Key)
}

func (t *T) write() (err error) {
	var f *os.File
	ini.DefaultHeader = true
	dir := filepath.Dir(t.ConfigFilePath)
	if err = os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	base := filepath.Base(t.ConfigFilePath)
	if f, err = os.CreateTemp(dir, "."+base+".*"); err != nil {
		return err
	}
	fName := f.Name()
	defer os.Remove(fName)

	ini.PrettyEqual = false
	ini.PrettyFormat = false
	ini.DefaultFormatLeft = " "
	ini.DefaultFormatRight = " "

	if _, err = t.file.WriteTo(f); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err := os.Rename(fName, t.ConfigFilePath); err != nil {
		return err
	}
	t.changed = false
	return nil
}

func (t *T) Eval(k key.T) (interface{}, error) {
	return t.EvalAs(k, "")
}

// EvalAs returns a key value,
//   - contextualized for a node (by default the local node, customized by the
//     impersonate option)
//   - dereferenced
//   - evaluated
func (t *T) EvalAs(k key.T, impersonate string) (interface{}, error) {
	if t == nil {
		return nil, errors.New("unreadable config")
	}
	switch k.Section {
	case "data", "env", "labels":
		return t.EvalKeywordAs(k, keywords.Keyword{}, impersonate)
	}
	sectionType := t.SectionType(k)
	kw, err := getKeyword(k, sectionType, t.Referrer)
	if err != nil {
		return nil, err
	}
	return t.EvalKeywordAs(k, kw, impersonate)
}

func (t *T) EvalNoConv(k key.T) (string, error) {
	return t.EvalAsNoConv(k, "")
}

func (t *T) EvalAsNoConv(k key.T, impersonate string) (string, error) {
	sectionType := t.SectionType(k)
	kw, err := getKeyword(k, sectionType, t.Referrer)
	if err != nil {
		return "", err
	}
	return t.evalStringAs(k, kw, impersonate)
}

func (t *T) SectionType(k key.T) string {
	if k.Option == "type" {
		return ""
	}
	return t.GetString(key.New(k.Section, "type"))
}

func (t *T) EvalKeywordAs(k key.T, kw keywords.Keyword, impersonate string) (interface{}, error) {
	v, err := t.evalStringAs(k, kw, impersonate)
	if err != nil {
		return nil, err
	}
	return t.convert(v, kw)
}

func getKeyword(k key.T, sectionType string, referrer Referrer) (keywords.Keyword, error) {
	var kw keywords.Keyword
	if referrer == nil {
		return kw, fmt.Errorf("%w: no referrer", ErrNoKeyword)
	}
	kw = referrer.KeywordLookup(k, sectionType)
	if kw.IsZero() {
		return kw, fmt.Errorf("%w: %s", ErrNoKeyword, k)
	}
	return kw, nil
}

func (t *T) evalStringAs(k key.T, kw keywords.Keyword, impersonate string) (string, error) {
	var (
		v   string
		err error
	)
	switch kw.Inherit {
	case keywords.InheritHead2Leaf:
		firstKey := kw.DefaultKey()
		if v, err = t.evalDescopeStringAs(firstKey, kw, impersonate); err == nil {
			return v, err
		}
		if v, err = t.evalDescopeStringAs(k, kw, impersonate); err == nil {
			return v, err
		}
	case keywords.InheritLeaf:
		if v, err = t.evalDescopeStringAs(k, kw, impersonate); err == nil {
			return v, err
		}
	default:
		if v, err = t.evalDescopeStringAs(k, kw, impersonate); err == nil {
			return v, err
		}
		firstKey := kw.DefaultKey()
		if v, err = t.evalDescopeStringAs(firstKey, kw, impersonate); err == nil {
			return v, err
		}
	}
	switch {
	case errors.Is(err, ErrExist):
		switch kw.Required {
		case true:
			return "", err
		case false:
			if kw.Default == "" {
				return "", nil
			}
			return t.replaceReferences(kw.Default, k.Section, impersonate)
		}
	case err != nil:
		return "", err
	}
	return v, nil
}

func (t *T) evalDescopeStringAs(k key.T, kw keywords.Keyword, impersonate string) (string, error) {
	v, err := t.mayDescope(k, kw, impersonate)
	if err != nil {
		return "", err
	}
	return t.replaceReferences(v, k.Section, impersonate)
}

func (t *T) convert(v string, kw keywords.Keyword) (interface{}, error) {
	if kw.Converter == nil {
		return v, nil
	}
	return kw.Converter.Convert(v)
}

func (t *T) mayDescope(k key.T, kw keywords.Keyword, impersonate string) (string, error) {
	var (
		v   string
		err error
	)
	if kw.Scopable {
		v, err = t.descope(k, kw, impersonate)
	} else {
		v, err = t.GetStrict(k)
	}
	return v, err
}

func (t *T) replaceReferences(v string, section string, impersonate string) (string, error) {
	var errs error
	v = rawconfig.RegexpReference.ReplaceAllStringFunc(v, func(ref string) string {
		s, err := t.dereference(ref, section, impersonate)
		if err != nil {
			switch err.(type) {
			case ErrPostponedRef:
				errs = errors.Join(errs, err)
			}
			return ref
		}
		return s
	})
	return v, errs
}

func (t T) SectionSig(section string) string {
	s, err := t.file.GetSection(section)
	if err != nil {
		return ""
	}
	sum := md5.New()
	for _, k := range s.Keys() {
		sum.Write([]byte(k.Value()))
	}
	return hex.EncodeToString(sum.Sum([]byte{}))
}

func (t T) SectionMap(section string) map[string]string {
	s, err := t.file.GetSection(section)
	if err != nil {
		return map[string]string{}
	}
	return s.KeysHash()
}

func (t T) SectionMapStrict(section string) (map[string]string, error) {
	s, err := t.file.GetSection(section)
	if err != nil {
		return nil, fmt.Errorf("%w: section '%s'", ErrExist, section)
	}
	return s.KeysHash(), nil
}

func (t *T) descope(k key.T, kw keywords.Keyword, impersonate string) (string, error) {
	if impersonate == "" {
		impersonate = hostname.Hostname()
	}
	s, err := t.SectionMapStrict(k.Section)
	if err != nil {
		return "", err
	}
	l := append(kw.Aliases, kw.Option)
	for _, o := range l {
		if v, ok := s[o+"@"+impersonate]; ok {
			return v, nil
		}
	}
	for _, o := range l {
		if v, ok := s[o+"@nodes"]; ok {
			if in, err := t.IsInNodes(impersonate); err != nil {
				return "", err
			} else if in {
				return v, nil
			}
		}
	}
	for _, o := range l {
		if v, ok := s[o+"@drpnodes"]; ok {
			if in, err := t.IsInDRPNodes(impersonate); err != nil {
				return "", err
			} else if in {
				return v, nil
			}
		}
	}
	for _, o := range l {
		if v, ok := s[o+"@encapnodes"]; ok {
			if in, err := t.IsInEncapNodes(impersonate); err != nil {
				return "", err
			} else if in {
				return v, nil
			}
		}
	}
	for _, o := range l {
		if v, ok := s[o]; ok {
			return v, nil
		}
	}
	return "", fmt.Errorf("%w: key '%s' not found (all scopes tried)", ErrExist, k)
}

func (t T) Raw() rawconfig.T {
	r := rawconfig.T{}
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

// Ops returns the list of <section>.<option>[@<scope>]=<value>
// This format is used by the volume pools framework.
func (t T) Ops() []string {
	l := make([]string, 0)
	for _, s := range t.file.Sections() {
		for k, v := range s.KeysHash() {
			op := fmt.Sprintf("%s.%s=%s", s.Name(), k, v)
			l = append(l, op)
		}
	}
	return l
}

func (t T) RawEvaluated() (rawconfig.T, error) {
	return t.RawEvaluatedAs("")
}

func (t T) RawEvaluatedAs(impersonate string) (rawconfig.T, error) {
	r := rawconfig.New()
	for _, s := range t.file.Sections() {
		sectionMap := *orderedmap.New()
		for k := range s.KeysHash() {
			_k := key.New(s.Name(), k)
			_k.Option = _k.BaseOption()
			if v, err := t.EvalAs(_k, impersonate); err != nil {
				return rawconfig.New(), fmt.Errorf("eval: %w", err)
			} else {
				sectionMap.Set(_k.Option, v)
			}
		}
		r.Data.Set(s.Name(), sectionMap)
	}
	return r, nil
}

func (t T) HasSectionString(s string) bool {
	for _, e := range t.SectionStrings() {
		if s == e {
			return true
		}
	}
	return false
}

// SectionStrings returns list of section names.
func (t T) SectionStrings() []string {
	return t.file.SectionStrings()
}

func (t *T) IsInNodes(impersonate string) (bool, error) {
	nodes, err := t.Referrer.Nodes()
	if err != nil {
		return false, err
	}
	return slices.Contains(nodes, impersonate), nil
}

func (t *T) IsInDRPNodes(impersonate string) (bool, error) {
	nodes, err := t.Referrer.DRPNodes()
	if err != nil {
		return false, err
	}
	return slices.Contains(nodes, impersonate), nil
}

func (t *T) IsInEncapNodes(impersonate string) (bool, error) {
	if i, ok := t.Referrer.(encapNodeser); ok {
		nodes, err := i.EncapNodes()
		if err != nil {
			return false, err
		}
		return slices.Contains(nodes, impersonate), nil
	} else {
		return false, nil
	}
}

func (t T) dereference(ref string, section string, impersonate string) (string, error) {
	type f func(string) string
	var (
		modifier f
		err      error
		count    bool
	)
	val := ""
	ref = ref[1 : len(ref)-1]
	l := strings.SplitN(ref, ":", 2)
	switch l[0] {
	case "upper":
		modifier = strings.ToUpper
		ref = l[1]
	case "lower":
		modifier = strings.ToLower
		ref = l[1]
	case "capitalize":
		modifier = xstrings.Capitalize
		ref = l[1]
	case "title":
		modifier = strings.Title
		ref = l[1]
	case "swapcase":
		modifier = xstrings.SwapCase
		ref = l[1]
	default:
		modifier = func(s string) string { return s }
	}
	if strings.HasPrefix(ref, "#") {
		count = true
		ref = ref[1:]
	}
	switch {
	case strings.HasPrefix(ref, "node."):
		if val, err = t.dereferenceNodeKey(ref, impersonate); err != nil {
			return ref, err
		}
	default:
		if val, err = t.dereferenceWellKnown(ref, section, impersonate); err != nil {
			return ref, err
		}
	}
	if count {
		n := len(strings.Fields(val))
		return fmt.Sprint(n), nil
	}
	return modifier(val), nil
}

func (t T) dereferenceNodeKey(ref string, impersonate string) (string, error) {
	//
	// Extract the key string relative to the node configuration
	// Examples:
	//   "node.env" => "env"
	//   "node.labels.az" => "labels.az"
	//   "node.env.az" => "env.az"
	//
	l := strings.SplitN(ref, ".", 2)
	nodeRef := l[1]

	// Use "node" as the implicit section instead of "DEFAULT"
	if !strings.Contains(nodeRef, ".") {
		nodeRef = "node." + nodeRef
	}

	nodeKey := key.Parse(nodeRef)
	sectionType := t.SectionType(nodeKey)
	kw, err := getKeyword(nodeKey, sectionType, t.NodeReferrer)
	if err != nil {
		return ref, err
	}

	// Filter on node key section
	switch nodeKey.Section {
	case "env", "labels", "node":
		// allow
	default:
		// deny
		return ref, fmt.Errorf("denied reference to node key %s", ref)
	}

	val, err := t.NodeReferrer.Config().evalStringAs(nodeKey, kw, impersonate)
	if err != nil {
		return ref, err
	}
	return val, nil
}

func (t T) dereferenceKey(ref string, section string, impersonate string) (string, error) {
	refKey := key.Parse(ref)
	if refKey.Section == "" {
		refKey.Section = section
	}
	key, err := t.file.Section(refKey.Section).GetKey(refKey.Option)
	if err != nil {
		return "", err
	}
	return t.replaceReferences(key.String(), refKey.Section, impersonate)
}

func (t T) dereferenceWellKnown(ref string, section string, impersonate string) (string, error) {
	if impersonate == "" {
		impersonate = hostname.Hostname()
	}
	if v, err := t.dereferenceKey(ref, section, impersonate); err == nil {
		return v, nil
	}
	switch ref {
	case "dns_janitor_major":
		return "3", nil
	case "nodename":
		return impersonate, nil
	case "short_nodename":
		return strings.SplitN(impersonate, ".", 2)[0], nil
	case "rid":
		return section, nil
	case "rindex":
		l := strings.SplitN(section, "#", 2)
		if len(l) != 2 {
			return section, nil
		}
		return l[1], nil
	case "rgroup":
		l := strings.SplitN(section, "#", 2)
		if len(l) != 2 {
			return section, nil
		}
		return l[0], nil
	case "svcmgr":
		return os.Args[0] + " svc", nil
	case "nodemgr":
		return os.Args[0] + " node", nil
	case "etc":
		return rawconfig.Paths.Etc, nil
	case "var":
		return rawconfig.Paths.Var, nil
	}
	if t.Referrer != nil {
		if v, err := t.Referrer.Dereference(ref); err == nil {
			return v, nil
		}
	}
	if v, err := t.EvalAsNoConv(key.New(section, ref), impersonate); err == nil {
		return v, nil
	}
	return ref, fmt.Errorf("unknown reference: %s", ref)
}

func (t *T) LoadRaw(configData rawconfig.T) error {
	t.changed = true
	file := ini.Empty()
	if configData.Data == nil {
		t.file = file
		return nil
	}
	for _, section := range configData.Data.Keys() {
		m, _ := configData.Data.Get(section)
		omap, ok := m.(orderedmap.OrderedMap)
		if !ok {
			return fmt.Errorf("invalid section in raw config format: %+v", m)
		}
		for _, option := range omap.Keys() {
			value, _ := omap.Get(option)
			var v string
			if value == nil {
				v = ""
			} else {
				v = value.(string)
			}
			file.Section(section).Key(option).SetValue(v)
		}
	}
	t.file = file
	return nil
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

func (t *T) rawCommit(configData rawconfig.T, configPath string, validate bool) error {
	if !t.changed {
		return nil
	}
	if !configData.IsZero() {
		if err := t.LoadRaw(configData); err != nil {
			return err
		}
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
		if alerts, err := t.Validate(); err != nil {
			return fmt.Errorf("abort config commit: %w", err)
		} else if alerts.HasError() {
			return fmt.Errorf("abort config commit: validation errors")
		}
	}
	if t.Referrer != nil && !t.Referrer.IsVolatile() {
		if err := t.write(); err != nil {
			return err
		}
	}
	//t.clearRefCache()
	if t.postCommit != nil {
		return t.postCommit()
	}
	return nil
}

func (t *T) Recommit() error {
	t.changed = true
	return t.rawCommit(rawconfig.T{}, "", true)
}

func (t *T) RecommitInvalid() error {
	t.changed = true
	return t.rawCommit(rawconfig.T{}, "", false)
}

func (t *T) Commit() error {
	return t.rawCommit(rawconfig.T{}, "", true)
}

func (t *T) CommitInvalid() error {
	return t.rawCommit(rawconfig.T{}, "", false)
}

func (t *T) CommitTo(configPath string) error {
	return t.rawCommit(rawconfig.T{}, configPath, true)
}

func (t *T) CommitToInvalid(configPath string) error {
	return t.rawCommit(rawconfig.T{}, configPath, false)
}

func (t *T) CommitData(configData rawconfig.T) error {
	t.changed = true
	return t.rawCommit(configData, "", true)
}

func (t *T) CommitDataTo(configData rawconfig.T, configPath string) error {
	t.changed = true
	return t.rawCommit(configData, configPath, true)
}

func (t *T) CommitDataToInvalid(configData rawconfig.T, configPath string) error {
	t.changed = true
	return t.rawCommit(configData, configPath, false)
}

// DeleteSections deletes sections from the config and commit changes
func (t *T) DeleteSections(sections ...string) error {
	if err := t.PrepareDeleteSections(sections...); err != nil {
		return err
	}
	return t.Commit()
}

// PrepareDeleteSections deletes sections from the config without committing changes.
func (t *T) PrepareDeleteSections(sections ...string) error {
	for _, section := range sections {
		if _, err := t.file.GetSection(section); err != nil {
			continue
		}
		t.file.DeleteSection(section)
		t.changed = true
	}
	return nil
}

func (t T) ModTime() time.Time {
	return file.ModTime(t.ConfigFilePath)
}

// PrepareSet applies key operations to config without committing changes.
func (t *T) PrepareSet(kops ...keyop.T) error {
	for _, op := range kops {
		if op.IsZero() {
			return fmt.Errorf("invalid set expression: %s", op)
		}
		if err := t.prepareSetKey(op); err != nil {
			return err
		}
	}
	return nil
}

// PrepareUpdate applies:
//
//	1- delete sections
//	2- unset keywords
//	3- apply key operations
//
// without committing changes.
func (t *T) PrepareUpdate(deleteSections []string, unsetKeys []key.T, keyOps []keyop.T) error {
	if err := t.PrepareDeleteSections(deleteSections...); err != nil {
		return err
	}
	if err := t.PrepareUnset(unsetKeys...); err != nil {
		return err
	}
	if err := t.PrepareSet(keyOps...); err != nil {
		return err
	}
	return nil
}

// Update applies:
//
//	1- delete sections
//	2- unset keywords
//	3- apply key operations
//
// and commit changes.
func (t *T) Update(deleteSections []string, unsetKeys []key.T, keyOps []keyop.T) error {
	if err := t.PrepareUpdate(deleteSections, unsetKeys, keyOps); err != nil {
		return err
	}
	return t.Commit()
}
