package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/opensvc/om3/util/render/palette"
)

type (
	kv struct {
		k string
		v interface{}
	}
)

func flatten(value any, lkey *bytes.Buffer, flattened *map[string]string) {
	if value == nil {
		return
	}

	// Use a type switch, which is much faster than reflection.
	switch v := value.(type) {
	case map[string]any:
		// Keep track of the builder's length to backtrack efficiently.
		originalLen := lkey.Len()
		for key, val := range v {
			// Write the key separator.
			if lkey.Len() > 0 {
				lkey.WriteByte('.')
			}

			// Handle special characters in the key.
			if strings.ContainsAny(key, ".#$/") || hasDigitPrefix(key) {
				lkey.WriteString(`"`)
				lkey.WriteString(key)
				lkey.WriteString(`"`)
			} else {
				lkey.WriteString(key)
			}

			flatten(val, lkey, flattened)

			// Reset the builder to its original state for the next key in the map.
			lkey.Truncate(originalLen)
		}
	case []any:
		originalLen := lkey.Len()
		for i, val := range v {
			// Append the slice index part, e.g., "[0]"
			lkey.WriteByte('[')
			lkey.WriteString(strconv.Itoa(i))
			lkey.WriteByte(']')

			flatten(val, lkey, flattened)

			// Reset builder for the next iteration.
			lkey.Truncate(originalLen)
		}
	// Add fast paths for primitive types to avoid json.Marshal.
	case string:
		(*flattened)[lkey.String()] = `"` + v + `"`
	case bool:
		(*flattened)[lkey.String()] = strconv.FormatBool(v)
	case float64:
		(*flattened)[lkey.String()] = strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		(*flattened)[lkey.String()] = strconv.Itoa(v)
	default:
		// Fallback for complex types not handled above.
		b, err := json.Marshal(value)
		if err == nil { // Only assign if marshaling succeeds.
			(*flattened)[lkey.String()] = string(b)
		}
	}
}

// Flatten accepts a nested struct and returns a flat struct with key like a."b/c".d[0].e
func Flatten(value any) map[string]string {
	flattened := make(map[string]string)
	var b bytes.Buffer
	flatten(value, &b, &flattened)
	return flattened
}

// SprintFlat accepts a JSON formatted byte array and returns the sorted
// "key = val" buffer
func SprintFlat(b []byte) string {
	var buf bytes.Buffer
	for _, e := range sprintFlatData(b) {
		buf.WriteString(e.k)
		buf.WriteString(" = ")
		buf.WriteString(fmt.Sprint(e.v))
		buf.WriteString("\n")
	}
	return buf.String()
}

func FprintFlat(w io.Writer, b []byte) {
	for _, e := range sprintFlatData(b) {
		fmt.Fprint(w, e.k)
		fmt.Fprint(w, " = ")
		fmt.Fprintln(w, e.v)
	}
}

func PrintFlat(b []byte) {
	FprintFlat(os.Stdout, b)
}

func SprintFlatColor(b []byte, colorize *palette.ColorPaletteFunc) string {
	if colorize == nil {
		colorize = palette.DefaultFuncPalette()
	}
	var buf bytes.Buffer
	for _, e := range sprintFlatData(b) {
		buf.WriteString(colorize.Primary(e.k))
		buf.WriteString(colorize.Primary(" = "))
		buf.WriteString(fmt.Sprintln(e.v))
	}
	return buf.String()
}

func sprintFlatData(b []byte) []kv {
	var data interface{}
	json.Unmarshal(b, &data)
	flattened := Flatten(data)
	l := make([]kv, len(flattened))
	i := 0
	for k, v := range flattened {
		l[i] = kv{k: k, v: v}
		i++
	}
	slices.SortFunc(l, func(i, j kv) int {
		return strings.Compare(i.k, j.k)
	})
	return l
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

type Delta struct {
	cache    map[string]map[string]string
	colorize *palette.ColorPaletteFunc
}

func NewDiff() *Delta {
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
