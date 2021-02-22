package patch

import (
	"fmt"
	"reflect"
)

// Render formats a strings representing the PatchSet for humans
func Render(p Type) string {
	path := ""
	for _, e := range p[0].([]interface{}) {
		ev := reflect.ValueOf(e)
		switch ev.Kind() {
		case reflect.Float64:
			path += fmt.Sprintf("[%d]", uint64(e.(float64)))
		case reflect.Int:
			path += fmt.Sprintf("[%d]", e.(int))
		default:
			path += fmt.Sprintf(".%s", e)
		}
	}
	var v string
	if len(p) == 1 {
		v = "(deleted)"
	} else {
		v = fmt.Sprint(p[1])
	}
	return fmt.Sprintln(" ", path, "=>", v)
}

// RenderSet formats a strings representing the PatchSet for humans
func RenderSet(ps SetType) string {
	s := ""
	for _, p := range ps {
		s += Render(p)
	}
	return s
}
