package unstructured

import (
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
	if i, ok := v.(unstructureder); !ok {
		return l, fmt.Errorf("%w: %s", ErrNoInterface, reflect.TypeOf(v))
	} else {
		l = append(l, i.Unstructured())
	}
	return l, nil
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
