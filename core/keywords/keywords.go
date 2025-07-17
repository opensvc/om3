package keywords

import (
	"embed"
	"fmt"
	"io"
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
	// Keyword represents a configuration option in an object or node configuration file
	Keyword struct {
		Section string
		Option  string
		Attr    string

		// Scopable means the keyword can have a different value on nodes, drpnodes, encapnodes or a specific node.
		Scopable bool

		// Required means the keyword mean be set, and thus disregards the default value.
		Required bool

		// Converter is the name of a registered routine converting a string into the keyword expected type.
		Converter string

		// Text is a text explaining the role of the keyword.
		Text string

		// DefaultText is a text explaining the default value.
		DefaultText string

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

func ParseInherit(s string) Inherit {
	switch s {
	case "leaf2head":
		return InheritLeaf2Head
	case "head2leaf":
		return InheritHead2Leaf
	case "leaf":
		return InheritLeaf
	case "head":
		return InheritHead
	default:
		return -1
	}
}

func NewText(fs embed.FS, path string) string {
	b, _ := fs.ReadFile(path)
	return string(b)
}

func ParseIndex(s string) Index {
	l := strings.SplitN(s, ".", 2)
	n := len(l)
	switch n {
	case 1:
		return Index{s, ""}
	case 2:
		return Index{l[0], l[1]}
	default:
		return Index{"", ""}
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

func (t Store) Doc(w io.Writer, kind naming.Kind, driver, kw string, depth int) error {
	depth += 1
	if kw != "" {
		if len(t) == 0 {
			return fmt.Errorf("keyword '%s' not found", kw)
		}
		return t[0].Doc(w, depth)
	}
	m := t.KeywordsByDriver(kind)
	if driver != "" {
		index := ParseIndex(driver)
		if i, ok := m[index]; ok {
			return driverDoc(w, i, index, kind, depth)
		} else {
			return fmt.Errorf("driver '%s' not found", driver)
		}
	}
	l := Indices(maps.Keys(m))
	sort.Sort(l)
	for _, index := range l {
		err := driverDoc(w, m[index], index, kind, depth)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t Store) DriverKeywords(section, typ string, kind naming.Kind) ([]Keyword, error) {
	index := Index{section, typ}
	m, ok := t.KeywordsByDriver(kind)[index]
	if !ok {
		return nil, fmt.Errorf("driver not found")
	}
	return maps.Values(m), nil
}

func driverDoc(w io.Writer, m map[string]Keyword, index Index, kind naming.Kind, depth int) error {
	section := index[0]
	typ := index[1]
	title := index.String()
	if title != "" {
		fmt.Fprintf(w, "%s %s\n\n", strings.Repeat("#", depth), index)
	}
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
		fmt.Fprint(w, "Minimal configlet:\n\n")
		if driver.NewGroup(section) == driver.GroupUnknown {
			fmt.Fprintf(w, "\t[%s]\n", section)
		} else {
			fmt.Fprintf(w, "\t[%s#1]\n", section)
		}
		fmt.Fprintln(w, strings.Join(cfgL, ""))
	}

	if len(cmdL) > 0 {
		fmt.Fprint(w, "Minimal setup command:\n\n")
		var selector string
		switch kind {
		case naming.KindInvalid:
			selector = "node"
		default:
			path := naming.Path{Namespace: "test", Kind: kind, Name: "foo"}
			selector = path.String()
		}
		if len(cmdL) > 1 {
			fmt.Fprintf(w, "\tom %s set \\\n\t\t%s\n\n", selector, strings.Join(cmdL, " \\\n\t\t"))
		} else {
			fmt.Fprintf(w, "\tom %s set %s\n\n", selector, cmdL[0])
		}
	}

	for _, opt := range optL {
		kw := m[opt]
		kw.Doc(w, depth)
		fmt.Fprintln(w, "")
	}
	return nil
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

func (t Keyword) IsZero() bool {
	return t.Option == ""
}

func (t Keyword) Doc(w io.Writer, depth int) error {
	fprintProp := func(a, b string) {
		fmt.Fprintf(w, "\t%-12s %s\n", a+":", b)
	}
	fmt.Fprintf(w, "%s %s\n\n", strings.Repeat("#", depth+1), t.Option)
	fprintProp("required", fmt.Sprint(t.Required))
	fprintProp("scopable", fmt.Sprint(t.Scopable))
	if len(t.Candidates) > 0 {
		fprintProp("candidates", strings.Join(t.Candidates, ", "))
	}
	if len(t.Depends) > 0 {
		l := make([]string, len(t.Depends))
		for i, kop := range t.Depends {
			l[i] = kop.String()
		}
		fprintProp("depends", strings.Join(l, ", "))
	}
	if t.DefaultText != "" {
		fprintProp("default", t.DefaultText)
	} else if t.Default != "" {
		fprintProp("default", t.Default)
	}
	if t.Converter != "" {
		fprintProp("convert", t.Converter)
	}
	fmt.Fprintln(w, "")
	if t.Example != "" {
		fmt.Fprintln(w, "Example:")
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "\t%s=%s\n\n", t.Option, t.Example)
	}
	fmt.Fprintln(w, t.Text)
	return nil
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
