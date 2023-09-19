package unstructured

import (
	"reflect"
)

type (
	List []Map
	Map  map[string]any

	unstructureder interface {
		Unstructured() map[string]any
	}
)

func Append(l List, v any) List {
	i := v.(unstructureder)
	l = append(l, i.Unstructured())
	return l
}

func NewList() List {
	l := make(List, 0)
	return l
}

func NewListWithData(data any) List {
	switch i := data.(type) {
	case List:
		return i
	}
	l := NewList()
	if data == nil {
		return l
	}
	switch reflect.TypeOf(data).Kind() {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(data)
		for i := 0; i < s.Len(); i++ {
			v := s.Index(i).Interface()
			l = Append(l, v)
		}
	default:
		l = Append(l, data)
	}
	return l
}
