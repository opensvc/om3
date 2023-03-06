package keywords

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/eidolon/wordwrap"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/stringslice"
	"github.com/pkg/errors"
	"github.com/ssrathi/go-attr"
	"golang.org/x/term"
)

// Keyword represents a configuration option in an object or node configuration file
type (
	Converter interface {
		Convert(string) (interface{}, error)
	}

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
		Kind kind.Mask

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

func (t Store) Lookup(k key.T, kd kind.T, sectionType string) Keyword {
	driverGroup := strings.Split(k.Section, "#")[0]
	baseOption := k.BaseOption()
	for _, kw := range t {
		if !kw.Kind.Has(kd) {
			continue
		}
		if baseOption != kw.Option && !stringslice.Has(baseOption, kw.Aliases) {
			continue
		}
		if kw.Section == "" {
			return kw
		}
		if sectionType != "" && len(kw.Types) > 0 && !stringslice.Has(sectionType, kw.Types) {
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

func (t Keyword) IsZero() bool {
	return t.Option == ""
}

func (t Keyword) Doc() string {
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if (err != nil) || (columns > 78) {
		columns = 78
	}
	pad := 12
	wrapper := wordwrap.Wrapper(columns-pad, false)
	fmt1 := func(a, b string) string {
		prefix := fmt.Sprintf("#   %-"+fmt.Sprintf("%d", pad)+"s", a+":")
		return wordwrap.Indent(wrapper(b), prefix, false) + "\n"
	}
	buff := "#\n"
	buff = buff + wordwrap.Indent(wrapper(t.Option), "# keyword:       ", false) + "\n"
	buff = buff + "# " + strings.Repeat("-", columns-2) + "\n"
	buff = buff + fmt1("required", fmt.Sprint(t.Required))
	buff = buff + fmt1("scopable", fmt.Sprint(t.Scopable))
	if len(t.Candidates) > 0 {
		buff = buff + fmt1("candidates", strings.Join(t.Candidates, ", "))
	}
	if len(t.Depends) > 0 {
		l := make([]string, len(t.Depends))
		for i, kop := range t.Depends {
			l[i] = kop.String()
		}
		buff = buff + fmt1("depends", strings.Join(l, ", "))
	}
	if t.DefaultText != "" {
		buff = buff + fmt1("default", t.DefaultText)
	} else if t.Default != "" {
		buff = buff + fmt1("default", t.Default)
	}
	if t.Converter != nil {
		buff = buff + fmt1("convert", fmt.Sprint(t.Converter))
	}
	buff = buff + "#\n"
	buff = buff + wordwrap.Indent(wordwrap.Wrapper(columns-4, false)(t.Text), "#   ", true) + "\n"
	buff = buff + "#\n"
	if t.Example != "" {
		buff = buff + ";" + t.Option + " = " + t.Example + "\n"
	}
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
			return errors.Wrapf(err, "set keyword %s: %s", t.Option, elements[i])
		}
	}
	if err := attr.SetValue(o, elements[n-1], v); err != nil {
		return errors.Wrapf(err, "set keyword %s: %s", t.Option, elements[n-1])
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
