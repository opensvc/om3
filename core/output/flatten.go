package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
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

// Flatten accepts a nested struct and returns a flat struct with key like a."b/c".d[0].e
func Flatten(inputJSON any) map[string]string {
	var lkey = ""
	var flattened = make(map[string]string)
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
	var data map[string]string
	json.Unmarshal(b, &data)
	flattened := Flatten(data)
	sprintFlattened(os.Stdout, flattened)
}

func sprintFlattened(w io.Writer, flattened map[string]string) {
	var keys []string
	for key, _ := range flattened {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println(k, "=", flattened[k])
	}
}

func hasDigitPrefix(s string) bool {
	if s == "" {
		return false
	}
	var r = ' '
	for _, r = range s {
		break
	}
	return r >= '0' && r <= '9'
}

func flatten(value any, lkey string, flattened *map[string]string) {
	v := reflect.ValueOf(value)
	if value == nil {
		return
	}
	switch v.Kind() {
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			k := fmt.Sprintf("%s[%d]", lkey, i)
			flatten(v.Index(i).Interface(), k, flattened)
		}
	case reflect.Map:
		for rkey, rval := range value.(map[string]interface{}) {
			if strings.ContainsAny(rkey, ".#$/") || hasDigitPrefix(rkey) {
				rkey = fmt.Sprintf("\"%s\"", rkey)
			}
			k := fmt.Sprintf("%s.%s", lkey, rkey)
			flatten(rval, k, flattened)
		}
	default:
		b, _ := json.Marshal(value)
		(*flattened)[lkey] = string(b)
	}
}

type Delta struct {
	cache    map[string]map[string]string
	colorize *palette.ColorPaletteFunc
}

func NewDelta() *Delta {
	return &Delta{
		colorize: palette.DefaultFuncPalette(),
		cache:    make(map[string]map[string]string),
	}
}

func (t *Delta) Key(data any) string {
	type keyer interface {
		Key() string
	}
	i, ok := data.(keyer)
	if !ok {
		return ""
	}
	return i.Key()
}

func (t *Delta) KeysToDelete(data any) []string {
	type keysToDeleter interface {
		KeysToDelete() []string
	}
	i, ok := data.(keysToDeleter)
	if !ok {
		return []string{}
	}
	return i.KeysToDelete()
}

func (t *Delta) Highlight(data any) []string {
	type highlighter interface {
		Highlight() []string
	}
	i, ok := data.(highlighter)
	if !ok {
		return []string{}
	}
	return i.Highlight()
}

func (t *Delta) Fprint(w io.Writer, data any) error {
	var m map[string]any
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	next := Flatten(m)
	key := t.Key(data)
	keysToDelete := t.KeysToDelete(data)
	highlight := t.Highlight(data)
	if len(keysToDelete) > 0 {
		for _, key := range keysToDelete {
			_, ok := t.cache[key]
			if !ok || key == "" {
				continue
			}
			delete(t.cache, key)
		}
		fmt.Fprintln(w, "---")
		for k, v := range next {
			if slices.Contains(highlight, k) {
				v = t.colorize.Bold(v)
			}
			fmt.Fprintf(w, " %s %s\n", t.colorize.Frozen(k+" ="), v)
		}
	} else {
		fmt.Fprintln(w, "---")
		last, ok := t.cache[key]
		if key == "" {
			// does not want caching, display in blue
			for k, v := range next {
				if slices.Contains(highlight, k) {
					v = t.colorize.Bold(v)
				}
				fmt.Fprintf(w, " %s %s\n", t.colorize.Frozen(k+" ="), v)
			}
		} else if !ok {
			// wants caching but not yet cached, display in green
			t.cache[key] = next
			for k, v := range next {
				s := t.colorize.Primary(k + " =")
				if slices.Contains(highlight, k) {
					v = t.colorize.Bold(v)
				}
				fmt.Fprintf(w, "%s%s %s\n", t.colorize.Optimal("+"), s, t.colorize.Optimal(v))
			}
		} else {
			allKeys := make([]string, 0)
			dedupedKeys := make(map[string]any)
			for key := range next {
				dedupedKeys[key] = nil
			}
			for key := range last {
				dedupedKeys[key] = nil
			}
			for key := range dedupedKeys {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)
			for _, k := range allKeys {
				lastValue, lastOk := last[k]
				nextValue, nextOk := next[k]
				s := t.colorize.Primary(k + " =")

				switch {
				case nextOk && !lastOk:
					if slices.Contains(highlight, k) {
						nextValue = t.colorize.Bold(nextValue)
					}
					fmt.Fprintf(w, "%s%s %s\n", t.colorize.Optimal("+"), s, t.colorize.Optimal(nextValue))
				case !nextOk && lastOk:
					if slices.Contains(highlight, k) {
						lastValue = t.colorize.Bold(lastValue)
					}
					fmt.Fprintf(w, "%s%s %s\n", t.colorize.Error("-"), s, t.colorize.Error(lastValue))
				case nextOk && lastOk:
					if lastValue != nextValue {
						if slices.Contains(highlight, k) {
							lastValue = t.colorize.Bold(lastValue)
						}
						fmt.Fprintf(w, "%s%s %s\n", t.colorize.Error("-"), s, t.colorize.Error(lastValue))
						if slices.Contains(highlight, k) {
							nextValue = t.colorize.Bold(nextValue)
						}
						fmt.Fprintf(w, "%s%s %s\n", t.colorize.Optimal("+"), s, t.colorize.Optimal(nextValue))
					} else {
						if slices.Contains(highlight, k) {
							nextValue = t.colorize.Bold(nextValue)
						}
						fmt.Fprintf(w, " %s %s\n", s, nextValue)
					}
				}
			}
			t.cache[key] = next
		}
	}
	return nil
}
