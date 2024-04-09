package keywords

import (
	"embed"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/key"
	"github.com/ssrathi/go-attr"
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
)

const (
	InheritLeaf2Head Inherit = iota
	InheritHead2Leaf
	InheritLeaf
	InheritHead
)

func NewText(fs embed.FS, path string) Text {
	return Text{fs, path}
}

func (t Text) String() string {
	if b, err := t.fs.ReadFile(t.path); err != nil {
		panic("missing documentation text file: " + t.path)
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
		if k.Section == kw.Section || driverGroup == kw.Section {
			return kw
		}
	}
	return Keyword{}
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

func (t Keyword) Doc() string {
	sprintProp := func(a, b string) string {
		return fmt.Sprintf("\t%-12s %s\n", a+":", b)
	}
	buff := "# " + t.Option + "\n\n"
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
