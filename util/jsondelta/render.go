package jsondelta

import (
	"fmt"
	"reflect"
	"strings"
)

// Render formats a strings representing a patch operation for humans
func (p OperationPath) String() string {
	path := ""
	for _, e := range p {
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
	path = strings.TrimLeft(path, ".")
	return path
}

// Render formats a strings representing a patch operation for humans
func (o Operation) Render() string {
	var v string
	if o.OpKind == "remove" {
		v = "(deleted)"
	} else if o.OpValue == nil {
		v = "null"
	} else {
		v = string(*o.OpValue)
	}
	return fmt.Sprintln(" ", o.OpPath, "=>", v)
}

// Render formats a strings representing the patch operations for humans
func (p Patch) Render() string {
	s := ""
	for _, o := range p {
		s += o.Render()
	}
	return s
}
