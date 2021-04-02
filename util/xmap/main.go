package xmap

import "reflect"

// Keys returns the slice of a map string keys.
func Keys(i interface{}) []string {
	m := reflect.ValueOf(i).MapKeys()
	l := make([]string, 0)
	for _, k := range m {
		l = append(l, k.String())
	}
	return l
}
