package rawconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/golang-collections/collections/set"
	"github.com/iancoleman/orderedmap"
)

type (
	T struct {
		Data *orderedmap.OrderedMap
	}
)

var (
	RegexpScope     = regexp.MustCompile(`(@[\w.-_]+)`)
	RegexpReference = regexp.MustCompile(`({[\w#.-_:]+})`)
)

func New() T {
	return T{
		Data: orderedmap.New(),
	}
}

// MarshalJSON marshals the enum as a quoted json string
func (t T) MarshalJSON() ([]byte, error) {
	return t.Data.MarshalJSON()
}

// UnmarshalJSON unmarshals a quoted json string to the enum value
func (t *T) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, &t.Data)
	if err != nil {
		return err
	}
	return nil
}

func (t T) Filter(keys []string) T {
	if len(keys) == 0 {
		return t
	}
	ks := append([]string{}, t.Data.Keys()...)
	for _, k := range ks {
		if !slices.Contains(keys, k) {
			t.Data.Delete(k)
		}
	}
	return t
}

// IsZero returns true if the Raw data has not been initialized
func (t T) IsZero() bool {
	return t.Data == nil
}

// Render return a colorized text version of the configuration file
func (t T) Render() string {
	return t.render(true)
}

func (t T) String() string {
	return t.render(false)
}

func (t T) render(colorize bool) string {
	buff := ""
	if t.Data == nil {
		return buff
	}
	for _, section := range t.Data.Keys() {
		if section == "metadata" {
			continue
		}
		s := fmt.Sprintf("[%s]\n", section)
		if colorize {
			s = Colorize.Primary(s)
		}
		buff += s
		data, _ := t.Data.Get(section)
		omap := data.(orderedmap.OrderedMap)
		for _, k := range omap.Keys() {
			v, _ := omap.Get(k)
			if k == "comment" {
				buff += renderComment(k, v)
				continue
			}
			buff += renderKey(k, v, colorize)
		}
		buff += "\n"
	}
	return buff
}

func renderComment(k string, v any) string {
	vs, ok := v.(string)
	if !ok {
		return ""
	}
	return "# " + strings.ReplaceAll(vs, "\n", "\n# ") + "\n"
}

func renderKey(k string, v any, colorize bool) string {
	if colorize {
		k = RegexpScope.ReplaceAllString(k, Colorize.Error("$1"))
	}
	var vs string
	switch o := v.(type) {
	case []any:
		l := make([]string, 0)
		for _, e := range o {
			if s, ok := e.(string); ok {
				l = append(l, s)
			}
		}
		vs = strings.Join(l, " ")
	case *set.Set:
		l := make([]string, 0)
		o.Do(func(e any) {
			if s, ok := e.(string); ok {
				l = append(l, s)
			}
		})
		vs = strings.Join(l, " ")
	case []string:
		vs = strings.Join(o, " ")
	case float64:
		vs = fmt.Sprintf("%f", o)
	case int, uint, int8, uint8, int64, uint64:
		vs = fmt.Sprintf("%d", o)
	case bool:
		vs = strconv.FormatBool(o)
	case string:
		vs = strings.ReplaceAll(o, "\n", "\n\t")
		if colorize {
			vs = RegexpReference.ReplaceAllString(o, Colorize.Optimal("$1"))
		}
	case fmt.Stringer:
		vs = o.String()
	default:
		//fmt.Println(o, reflect.TypeOf(o))
		vs = ""
	}
	if colorize {
		k = Colorize.Secondary(k)
	}
	return fmt.Sprintf("%s = %s\n", k, vs)
}
