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

func Copy[K, V comparable](m map[K]V) map[K]V {
	result := make(map[K]V)
	for k, v := range m {
		result[k] = v
	}
	return result
}
