package keywords

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/ssrathi/go-attr"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/stringslice"
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

		// Candidates is the list of accepted values. An empty list.
		Candidates []string

		// Depends is a list of key-value conditions to meet to accept this keyword in a config.
		//Depends []keyval.T

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
	}

	Store   []Keyword
	Inherit int
)

const (
	InheritLeaf2Head Inherit = iota
	InheritHead2Leaf
	InheritLeaf
)

func (t Store) Lookup(k key.T, kd kind.T, sectionType string) Keyword {
	driverGroup := strings.Split(k.Section, "#")[0]
	for _, kw := range t {
		if !kw.Kind.Has(kd) {
			continue
		}
		if k.Option != kw.Option && !stringslice.Has(k.Option, kw.Aliases) {
			continue
		}
		if kw.Section == "" {
			return kw
		}
		if sectionType != "" && !stringslice.Has(sectionType, kw.Types) {
			continue
		}
		if k.Section == kw.Section || driverGroup == kw.Section {
			return kw
		}
	}
	return Keyword{}
}

func (t Keyword) IsZero() bool {
	return t.Option == ""
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
