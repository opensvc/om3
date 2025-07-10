package keywords

import (
	"embed"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/key"
	"github.com/ssrathi/go-attr"
	"golang.org/x/exp/maps"
)

type (
	Converter interface {
		Convert(string) (interface{}, error)
	}

	Text struct {
		fs   embed.FS
		path string
	}

	// Keyword represents a configuration option in an object or node configuration file
	Keyword struct {
		Section string
		Option  string
		Attr    string

		// Scopable means the keyword can have a different value on nodes, drpnodes, encapnodes or a specific node.
		Scopable bool

		// Required means the keyword mean be set, and thus disregards the default value.
		Required bool

		// Converter is the routine converting from string a the keyword expected type.
		Converter Converter

		// Text is a text explaining the role of the keyword.
		Text Text

		// DefaultText is a text explaining the default value.
		DefaultText Text

		// Example demonstrates the keyword usage.
		Example string

		// Default is the value returned when the non-required keyword is not set.
		Default string

		// DefaultOption is the name of the option looked up in the
		// DEFAULT section if the keyword is not set. If not set,
		// the string in the Option field is looked up in the DEFAULT
		// section.
		DefaultOption string

		// Candidates is the list of accepted values. An empty list.
		Candidates []string

		// Depends is a list of key-value conditions to meet to accept this keyword in a config.
		Depends []keyop.T

		// Kind limits the scope of this keyword to the object with kind matching this mask.
		Kind naming.Kinds

		// Provisioning is set to true for keywords only used for resource provisioning
		Provisioning bool

		// Types limits the scope of the keyword to sections with matching type value
		Types []string

		// Aliases defines alternate names of the keyword.
		Aliases []string

		// Inherit defines weither DEFAULT.<name> overrides <rid>.<name> (Head2Leaf), or
		// <rid>.<name> overrides DEFAULT.<name> (Leaf2Head, the default), or only <rid>.<name>
		// is used (Leaf).
		Inherit Inherit

		// Deprecated is the release where the keyword has been deprecated. Users can
		// expect the keyword to be unsupported in the next release.
		Deprecated string

		// ReplacedBy means the keyword is deprecated but another keyword can be used instead.
		ReplacedBy string
	}

	Store   []Keyword
	Inherit int

	Index   [2]string
	Indices []Index
)

const (
	InheritLeaf2Head Inherit = iota
	InheritHead2Leaf
	InheritLeaf
	InheritHead
)

func (t Inherit) String() string {
	switch t {
	case InheritLeaf2Head:
		return "leaf2head"
	case InheritHead2Leaf:
		return "head2leaf"
	case InheritLeaf:
		return "leaf"
	case InheritHead:
		return "head"
	default:
		return "unknown"
	}
}

func NewText(fs embed.FS, path string) Text {
	return Text{fs, path}
}

func ParseIndex(s string) Index {
	l := strings.SplitN(s, ".", 2)
	if len(l) == 1 {
		return Index{s, ""}
	} else {
		return Index{l[0], l[1]}
	}
}

func (t Index) String() string {
	if t[1] == "" {
		return t[0]
	}
	return t[0] + "." + t[1]
}

func (t Indices) Len() int {
	return len(t)
}

func (t Indices) Less(i, j int) bool {
	if t[i][0] != t[j][0] {
		return t[i][0] < t[j][0]
	}
	return t[i][1] < t[j][1]
}

func (t Indices) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t Text) String() string {
	if b, err := t.fs.ReadFile(t.path); err != nil {
		return "TODO"
	} else {
		return string(b)
	}
}

// Name is a func required by the resource manifest Attr interface
func (t Keyword) Name() string {
	return t.Attr
}

func (t Store) Len() int {
	return len(t)
}

func (t Store) Less(i, j int) bool {
	if t[i].Section < t[j].Section {
		return true
	} else if t[i].Section > t[j].Section {
		return false
	}
	return t[i].Option < t[j].Option
}

func (t Store) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t Store) Lookup(k key.T, kind naming.Kind, sectionType string) Keyword {
	driverGroup := strings.Split(k.Section, "#")[0]
	baseOption := k.BaseOption()
	for _, kw := range t {
		if !kw.Kind.Has(kind) {
			continue
		}
		if baseOption != kw.Option && !slices.Contains(kw.Aliases, baseOption) {
			continue
		}
		if kw.Section == "" {
			return kw
		}
		if sectionType != "" && len(kw.Types) > 0 && !slices.Contains(kw.Types, sectionType) {
			continue
		}
		if k.Section == "*" || k.Section == kw.Section || driverGroup == kw.Section {
			return kw
		}
	}
	return Keyword{}
}

func (t Store) Doc(kind naming.Kind, depth int) (string, error) {
	depth += 1
	buff := ""
	m := t.KeywordsByDriver(kind)
	l := Indices(maps.Keys(m))
	sort.Sort(l)
	for _, index := range l {
		s, err := driverDoc(m[index], index[0], index[1], kind, depth)
		if err != nil {
			return buff, err
		}
		buff += s
	}
	return buff, nil
}

func (t Store) KeywordDoc(section, typ, option string, kind naming.Kind, depth int) (string, error) {
	kw := t.Lookup(key.T{Section: section, Option: option}, kind, typ)
	if kw.IsZero() {
		return "", fmt.Errorf("keyword not found")
	}
	return kw.Doc(depth), nil
}

func (t Store) DriverDoc(section, typ string, kind naming.Kind, depth int) (string, error) {
	depth += 1
	index := Index{section, typ}
	m, ok := t.KeywordsByDriver(kind)[index]
	if !ok {
		return "", fmt.Errorf("driver not found")
	}
	return driverDoc(m, section, typ, kind, depth)
}

func driverDoc(m map[string]Keyword, section, typ string, kind naming.Kind, depth int) (string, error) {
	index := Index{section, typ}
	buff := fmt.Sprintf("%s %s\n\n", strings.Repeat("#", depth), index)
	optL := maps.Keys(m)
	sort.Strings(optL)

	cfgL := make([]string, 0)
	cmdL := make([]string, 0)

	if typ != "" {
		cfgL = append(cfgL, fmt.Sprintf("\t%s = %s\n", "type", typ))
		cmdL = append(cmdL, fmt.Sprintf("--kw=\"%s=%s\"", "type", typ))
	}

	for _, name := range optL {
		kw := m[name]
		if name == "type" {
			continue
		}
		if !kw.Required {
			continue
		}
		cfgL = append(cfgL, fmt.Sprintf("\t%s = %s\n", kw.Option, kw.Example))
		cmdL = append(cmdL, fmt.Sprintf("--kw=\"%s=%s\"", kw.Option, kw.Example))
	}

	if len(cfgL) > 0 {
		buff += fmt.Sprint("Minimal configlet:\n\n")
		if driver.NewGroup(section) == driver.GroupUnknown {
			buff += fmt.Sprintf("\t[%s]\n", section)
		} else {
			buff += fmt.Sprintf("\t[%s#1]\n", section)
		}
		buff += strings.Join(cfgL, "") + "\n"
	}

	if len(cmdL) > 0 {
		buff += fmt.Sprint("Minimal setup command:\n\n")
		var selector string
		switch kind {
		case naming.KindInvalid:
			selector = "node"
		default:
			path := naming.Path{Namespace: "test", Kind: kind, Name: "foo"}
			selector = path.String()
		}
		if len(cmdL) > 1 {
			buff += fmt.Sprintf("\tom %s set \\\n\t\t", selector) + strings.Join(cmdL, " \\\n\t\t") + "\n\n"
		} else {
			buff += fmt.Sprintf("\tom %s set %s\n\n", selector, cmdL[0])
		}
	}

	for _, opt := range optL {
		kw := m[opt]
		buff += kw.Doc(depth)
		buff += "\n"
	}
	return buff, nil
}

func (t Store) KeywordsByDriver(kind naming.Kind) map[Index]map[string]Keyword {
	m := make(map[Index]map[string]Keyword)
	sections := make(map[string]any)
	typesByGroup := make(map[string][]string)
	do := func(kw Keyword) {
		var types []string
		if len(kw.Types) > 0 {
			types = kw.Types
		} else if l, ok := typesByGroup[kw.Section]; ok {
			types = append(types, l...)
		} else if driver.NewGroup(kw.Section) == driver.GroupUnknown {
			types = append(types, "")
		}
		for _, typ := range types {
			key := Index{kw.Section, typ}
			if _, ok := m[key]; !ok {
				m[key] = make(map[string]Keyword)
			}
			m[key][kw.Option] = kw
		}
	}
	for _, kw := range t {
		if kw.Section != "" && kw.Kind.Has(kind) {
			sections[kw.Section] = nil
		}
	}
	for group, names := range driver.NamesByGroup() {
		typesByGroup[group.String()] = names
	}
	for _, kw := range t {
		if !kw.Kind.Has(kind) {
			continue
		}
		var l []string
		if kw.Section == "" {
			for section, _ := range sections {
				l = append(l, section)
			}
		} else {
			l = append(l, kw.Section)
		}
		for _, section := range l {
			kw.Section = section
			do(kw)
		}
	}
	return m
}

func (t Keyword) DefaultKey() key.T {
	k := key.T{
		Section: "DEFAULT",
		Option:  t.Option,
	}
	if t.DefaultOption != "" {
		k.Option = t.DefaultOption
	}
	return k
}

func (t Text) IsZero() bool {
	return t.path == ""
}

func (t Keyword) IsZero() bool {
	return t.Option == ""
}

func (t Keyword) Doc(depth int) string {
	sprintProp := func(a, b string) string {
		return fmt.Sprintf("\t%-12s %s\n", a+":", b)
	}
	buff := fmt.Sprintf("%s %s\n\n", strings.Repeat("#", depth+1), t.Option)
	buff += sprintProp("required", fmt.Sprint(t.Required))
	buff += sprintProp("scopable", fmt.Sprint(t.Scopable))
	if len(t.Candidates) > 0 {
		buff += sprintProp("candidates", strings.Join(t.Candidates, ", "))
	}
	if len(t.Depends) > 0 {
		l := make([]string, len(t.Depends))
		for i, kop := range t.Depends {
			l[i] = kop.String()
		}
		buff += sprintProp("depends", strings.Join(l, ", "))
	}
	if !t.DefaultText.IsZero() {
		buff += sprintProp("default", t.DefaultText.String())
	} else if t.Default != "" {
		buff += sprintProp("default", t.Default)
	}
	if t.Converter != nil {
		buff += sprintProp("convert", fmt.Sprint(t.Converter))
	}
	buff += "\n"
	if t.Example != "" {
		buff += "Example:\n"
		buff += "\n"
		buff += "\t" + t.Option + " = " + t.Example + "\n"
		buff += "\n"
	}
	buff += t.Text.String()
	buff += "\n"
	return buff
}

func (t *Keyword) SetValue(r, v interface{}) error {
	elements := strings.Split(t.Attr, ".")
	n := len(elements)
	if n == 0 {
		return fmt.Errorf("set keyword %s: no Attr in keyword definition", t.Option)
	}
	o := r
	var err error
	for i := 0; i < n-1; i = i + 1 {
		o, err = getValueAddr(o, elements[i])
		if err != nil {
			return fmt.Errorf("set keyword %s=%s: %w", t.Option, elements[i], err)
		}
	}
	if err := attr.SetValue(o, elements[n-1], v); err != nil {
		return fmt.Errorf("set keyword %s = %s: %w", t.Option, elements[n-1], err)
	}
	return nil
}

func getReflectValue(obj interface{}) (reflect.Value, error) {
	value := reflect.ValueOf(obj)

	if value.Kind() == reflect.Struct {
		return value, nil
	}

	if value.Kind() == reflect.Ptr && value.Elem().Kind() == reflect.Struct {
		return value.Elem(), nil
	}

	var retval reflect.Value
	return retval, attr.ErrNotStruct
}

func getValueAddr(obj interface{}, fieldName string) (interface{}, error) {
	objValue, err := getReflectValue(obj)
	if err != nil {
		return nil, err
	}

	fieldValue := objValue.FieldByName(fieldName)
	if !fieldValue.IsValid() {
		return nil, attr.ErrNoField
	}

	if !fieldValue.CanInterface() {
		return nil, attr.ErrUnexportedField
	}

	return fieldValue.Addr().Interface(), nil
}
