package rawconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
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

// IsZero returns true if the Raw data has not been initialized
func (t T) IsZero() bool {
	return t.Data == nil
}

// Render return a colorized text version of the configuration file
func (t T) Render() string {
	s := ""
	if t.Data == nil {
		return s
	}
	for _, section := range t.Data.Keys() {
		if section == "metadata" {
			continue
		}
		s += Colorize.Primary(fmt.Sprintf("[%s]\n", section))
		data, _ := t.Data.Get(section)
		omap := data.(orderedmap.OrderedMap)
		for _, k := range omap.Keys() {
			v, _ := omap.Get(k)
			if k == "comment" {
				s += renderComment(k, v)
				continue
			}
			s += renderKey(k, v)
		}
		s += "\n"
	}
	return s
}

func renderComment(k string, v any) string {
	vs, ok := v.(string)
	if !ok {
		return ""
	}
	return "# " + strings.ReplaceAll(vs, "\n", "\n# ") + "\n"
}

func renderKey(k string, v any) string {
	k = RegexpScope.ReplaceAllString(k, Colorize.Error("$1"))
	var vs string
	type stringer interface {
		String() string
	}
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
		vs = RegexpReference.ReplaceAllString(o, Colorize.Optimal("$1"))
		vs = strings.ReplaceAll(vs, "\n", "\n\t")
	case stringer:
		vs = o.String()
	default:
		//fmt.Println(o, reflect.TypeOf(o))
		vs = ""
	}
	return fmt.Sprintf("%s = %s\n", Colorize.Secondary(k), vs)
}
