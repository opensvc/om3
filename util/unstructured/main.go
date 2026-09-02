package unstructured

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

type (
	List []Map
	Map  map[string]any

	unstructureder interface {
		Unstructured() map[string]any
	}
)

var (
	ErrNoInterface = errors.New("unstructured interface is not implemented")
)

func Append(l List, v any) List {
	i := v.(unstructureder)
	l = append(l, i.Unstructured())
	return l
}

func AppendStrict(l List, v any) (List, error) {
	if i, ok := v.(unstructureder); ok {
		return append(l, i.Unstructured()), nil
	}
	m, err := fromJSON(v)
	if err != nil {
		return l, fmt.Errorf("%w: %s: %s", ErrNoInterface, reflect.TypeOf(v), err)
	}
	return append(l, m), nil
}

// fromJSON returns the map a value marshals to, keyed by the json names
// its tags declare.
//
// This is for the readers that need a map and cannot walk a struct, the
// go templates of the template renderer among them. The numbers are
// decoded as json.Number rather than as float64, so that a large int64,
// a size in bytes for example, prints its digits and not an exponent.
func fromJSON(v any) (Map, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	m := make(Map)
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func NewList() List {
	l := make(List, 0)
	return l
}

func NewListWithData(data any) (List, error) {
	var err error
	switch i := data.(type) {
	case List:
		return i, nil
	}
	l := NewList()
	if data == nil {
		return l, nil
	}
	switch reflect.TypeOf(data).Kind() {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(data)
		for i := 0; i < s.Len(); i++ {
			v := s.Index(i).Interface()
			l, err = AppendStrict(l, v)
		}
	default:
		l, err = AppendStrict(l, data)
	}
	return l, err
}
