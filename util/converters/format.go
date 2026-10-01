package converters

import (
	"fmt"
	"os"
	"os/user"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-collections/collections/set"
)

// Formatter is a converter writing a value it converted back the way a
// configuration writes it: a duration as 2m, a size as 5g.
//
// The converted value is what a program reads, a count of nanoseconds or of
// bytes. The text is what a person reads, and converts back to the same
// value.
type Formatter interface {
	Format(v any) string
}

// Format writes the value the converter c converted the way a configuration
// writes it. A converter with no Formatter, and no converter at all, write
// the value as it is: a list as its words, a pointer as what it points to,
// nothing as the empty text.
func Format(c Converter, v any) string {
	if v == nil {
		return ""
	}
	if f, ok := c.(Formatter); ok {
		return f.Format(v)
	}
	return formatValue(v)
}

func formatValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []string:
		return strings.Join(t, " ")
	case *set.Set:
		if t == nil {
			return ""
		}
		l := make([]string, 0, t.Len())
		t.Do(func(e any) {
			l = append(l, fmt.Sprint(e))
		})
		sort.Strings(l)
		return strings.Join(l, " ")
	case fmt.Stringer:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && rv.IsNil() {
			return ""
		}
		return t.String()
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		return formatValue(rv.Elem().Interface())
	}
	return fmt.Sprint(v)
}

// Format writes a duration with its zero units dropped, so a duration
// configured as 2m reads 2m, and not the 2m0s of time.Duration.
func (t TDuration) Format(v any) string {
	var d time.Duration
	switch t := v.(type) {
	case *time.Duration:
		if t == nil {
			return ""
		}
		d = *t
	case time.Duration:
		d = t
	default:
		return formatValue(v)
	}
	return FormatDuration(d)
}

// FormatDuration writes a duration with its zero units dropped: 2m, 1h30m,
// 1h, 500ms, 0s.
func FormatDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}

// Format writes a size in the largest unit it is a whole number of, so it
// converts back to the same number of bytes: 5g, 1536m, and a count of bytes
// when no unit divides it.
func (t TSize) Format(v any) string {
	var n int64
	switch t := v.(type) {
	case *int64:
		if t == nil {
			return ""
		}
		n = *t
	case int64:
		n = t
	default:
		return formatValue(v)
	}
	return FormatSize(n)
}

// FormatSize writes a number of bytes in the largest binary unit it is a
// whole number of.
func FormatSize(n int64) string {
	units := []string{"", "k", "m", "g", "t", "p", "e"}
	i := 0
	for i < len(units)-1 && n != 0 && n%1024 == 0 {
		n /= 1024
		i++
	}
	return strconv.FormatInt(n, 10) + units[i]
}

// Format writes the words of a command line, quoting the ones a shell would
// split.
func (t TShlex) Format(v any) string {
	l, ok := v.([]string)
	if !ok {
		return formatValue(v)
	}
	quoted := make([]string, len(l))
	for i, word := range l {
		if word == "" || strings.ContainsAny(word, " \t\n\"'\\$`") {
			quoted[i] = strconv.Quote(word)
		} else {
			quoted[i] = word
		}
	}
	return strings.Join(quoted, " ")
}

// Format writes a umask in octal, as it is configured.
func (t TUmask) Format(v any) string {
	return formatMode(v, "%03o")
}

// Format writes a file mode in octal, as it is configured, with the setuid,
// setgid and sticky bits as the leading digit when one is set.
func (t TFileMode) Format(v any) string {
	m, ok := v.(*os.FileMode)
	if !ok || m == nil {
		return formatMode(v, "%03o")
	}
	special := 0
	if *m&os.ModeSetuid != 0 {
		special |= 4
	}
	if *m&os.ModeSetgid != 0 {
		special |= 2
	}
	if *m&os.ModeSticky != 0 {
		special |= 1
	}
	if special != 0 {
		return fmt.Sprintf("%d%03o", special, uint32(m.Perm()))
	}
	return fmt.Sprintf("%03o", uint32(m.Perm()))
}

func formatMode(v any, format string) string {
	switch t := v.(type) {
	case *os.FileMode:
		if t == nil {
			return ""
		}
		return fmt.Sprintf(format, uint32(*t))
	case os.FileMode:
		return fmt.Sprintf(format, uint32(t))
	}
	return formatValue(v)
}

// Format writes a user as its name.
func (t TUser) Format(v any) string {
	if u, ok := v.(*user.User); ok {
		if u == nil {
			return ""
		}
		return u.Username
	}
	return formatValue(v)
}

// Format writes a group as its name.
func (t TGroup) Format(v any) string {
	if g, ok := v.(*user.Group); ok {
		if g == nil {
			return ""
		}
		return g.Name
	}
	return formatValue(v)
}
