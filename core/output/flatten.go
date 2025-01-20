package output

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/opensvc/om3/util/render/palette"
)

type (
	kv struct {
		k string
		v interface{}
	}
)

// Flatten accepts a nested struct and returns a flat struct with key like a.'b/c'.d[0].e
func Flatten(inputJSON interface{}) map[string]interface{} {
	var lkey = ""
	var flattened = make(map[string]interface{})
	flatten(inputJSON, lkey, &flattened)
	return flattened
}

// SprintFlat accepts a JSON formatted byte array and returns the sorted
// "key = val" buffer
func SprintFlat(b []byte) string {
	s := ""
	for _, e := range sprintFlatData(b) {
		s += fmt.Sprintln(e.k+" =", e.v)
	}
	return s
}

func SprintFlatColor(b []byte, colorize *palette.ColorPaletteFunc) string {
	if colorize == nil {
		colorize = palette.DefaultFuncPalette()
	}
	s := ""
	for _, e := range sprintFlatData(b) {
		s += fmt.Sprintln(colorize.Primary(e.k+" ="), e.v)
	}
	return s
}

func sprintFlatData(b []byte) []kv {
	var data interface{}
	json.Unmarshal(b, &data)
	flattened := Flatten(data)
	keys := make([]string, 0)
	for key := range flattened {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	l := make([]kv, len(keys))
	for i, k := range keys {
		l[i] = kv{k: k, v: flattened[k]}
	}
	return l
}

// PrintFlat accepts a JSON formatted byte array and prints to stdout the sorted
// "key = val"
func PrintFlat(b []byte) {
	var data map[string]interface{}
	json.Unmarshal(b, &data)
	flattened := Flatten(data)
	keys := make([]string, 0)
	for key := range flattened {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println(k, "=", flattened[k])
	}
}

func flatten(value interface{}, lkey string, flattened *map[string]interface{}) {
	v := reflect.ValueOf(value)
	if value == nil {
		return
	}
	switch v.Kind() {
	case reflect.Slice:
		for i := 0; i < len(value.([]interface{})); i++ {
			k := fmt.Sprintf("%s[%d]", lkey, i)
			flatten(value.([]interface{})[i], k, flattened)
		}
	case reflect.Map:
		for rkey, rval := range value.(map[string]interface{}) {
			if strings.ContainsAny(rkey, ".#$/") {
				rkey = fmt.Sprintf("'%s'", rkey)
			}
			k := fmt.Sprintf("%s.%s", lkey, rkey)
			flatten(rval, k, flattened)
		}
	default:
		b, _ := json.Marshal(value)
		(*flattened)[lkey] = string(b)
	}
}
