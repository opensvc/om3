package converters

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/util/sizeconv"

	"github.com/anmitsu/go-shlex"
	"github.com/golang-collections/collections/set"
)

type (
	TString        struct{}
	TInt           struct{}
	TInt64         struct{}
	TFloat64       struct{}
	TBool          struct{}
	TList          struct{}
	TListLowercase struct{}
	TSet           struct{}
	TShlex         struct{}
	TDuration      struct{}
	TUmask         struct{}
	TSize          struct{}
	TFileMode      struct{}
	TTristate      struct{}

	Converter interface {
		Convert(s string) (any, error)
		String() string
	}
)

var (
	DB map[string]Converter
)

func init() {
	DB = make(map[string]Converter)
	DB[""] = TString{}
	Register(TString{})
	Register(TInt{})
	Register(TInt64{})
	Register(TFloat64{})
	Register(TBool{})
	Register(TList{})
	Register(TListLowercase{})
	Register(TSet{})
	Register(TShlex{})
	Register(TDuration{})
	Register(TUmask{})
	Register(TSize{})
	Register(TFileMode{})
	Register(TTristate{})
}

func Register(c Converter) {
	DB[c.String()] = c
}

func Lookup(s string) Converter {
	c, ok := DB[s]
	if !ok {
		panic(fmt.Sprintf("converter '%s' is not registered", s))
	}
	return c
}

func (t TTristate) Convert(s string) (any, error) {
	if s == "" {
		return "", nil
	}
	s = strings.TrimSpace(s)
	v, err := strconv.ParseBool(s)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(v), nil
}

func (t TTristate) String() string {
	return "tristate"
}

func (t TString) Convert(s string) (any, error) {
	return s, nil
}

func (t TString) String() string {
	return "string"
}

func (t TInt) Convert(s string) (any, error) {
	if i, err := strconv.Atoi(s); err != nil {
		//fmt.Println(string(debug.Stack()))
		return 0, fmt.Errorf("int convert error: %s", err)
	} else {
		return i, nil
	}
}

func (t TInt) String() string {
	return "int"
}

func (t TInt64) Convert(s string) (any, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (t TInt64) String() string {
	return "int64"
}

func (t TFloat64) Convert(s string) (any, error) {
	return strconv.ParseFloat(s, 64)
}

func (t TFloat64) String() string {
	return "float64"
}

func (t TBool) Convert(s string) (any, error) {
	if s == "" {
		return false, nil
	}
	s = strings.TrimSpace(s)
	return strconv.ParseBool(s)
}

func (t TBool) String() string {
	return "bool"
}

func (t TList) Convert(s string) (any, error) {
	return strings.Fields(s), nil
}

func (t TList) String() string {
	return "list"
}

func (t TListLowercase) Convert(s string) (any, error) {
	l := strings.Fields(s)
	for i := 0; i < len(l); i++ {
		l[i] = strings.ToLower(l[i])
	}
	return l, nil
}

func (t TListLowercase) String() string {
	return "listlowercase"
}

func (t TSet) Convert(s string) (any, error) {
	aSet := set.New()
	for _, e := range strings.Fields(s) {
		aSet.Insert(e)
	}
	return aSet, nil
}

func (t TSet) String() string {
	return "set"
}

func (t TShlex) Convert(s string) (any, error) {
	return shlex.Split(s, true)
}

func (t TShlex) String() string {
	return "shlex"
}

// Convert converts duration string to *time.Duration
//
// nil is returned when duration is unset
// Default unit is second when not specified
func (t TDuration) Convert(s string) (any, error) {
	return t.convert(s)
}

// DurationWithDefaultMinMax parses a duration string and clamps the result within minD and maxD or returns defaultD.
// If s is nil or empty, defaultD is returned, clamped within the range defined by minD and maxD.
func DurationWithDefaultMinMax(s *string, defaultD, minD, maxD time.Duration) (time.Duration, error) {
	return TDuration{}.convertWithDefaultMinMax(s, defaultD, minD, maxD)
}

func (t TDuration) convertWithDefaultMinMax(s *string, defaultD, minD, maxD time.Duration) (time.Duration, error) {
	if s == nil || *s == "" {
		return min(maxD, max(minD, defaultD)), nil
	}
	d, err := t.convert(*s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration conversion %s: %w", *s, err)
	} else if d == nil {
		return min(maxD, max(minD, defaultD)), nil
	} else {
		return max(minD, min(maxD, *d)), nil
	}
}

func (t TDuration) convert(s string) (*time.Duration, error) {
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.Atoi(s); err == nil {
		s = s + "s"
	}
	duration, err := ParseDuration(s)
	if err != nil {
		return nil, err
	}
	return &duration, nil
}

func (t TDuration) String() string {
	return "duration"
}

func (t TUmask) Convert(s string) (any, error) {
	return t.convert(s)
}

func (t TUmask) convert(s string) (*os.FileMode, error) {
	if s == "" {
		return nil, nil
	}
	i, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return nil, errors.New("unexpected umask value: " + s + " " + err.Error())
	}
	umask := os.FileMode(i)
	return &umask, nil
}

func (t TUmask) String() string {
	return "umask"
}

func (t TSize) Convert(s string) (any, error) {
	return t.convert(s)
}

func (t TSize) convert(s string) (*int64, error) {
	var (
		err error
		i   int64
	)
	if s == "" {
		return nil, err
	}
	if strings.Contains(s, "%") {
		return nil, err
	}
	if i, err = sizeconv.FromSize(s); err != nil {
		return nil, err
	}
	return &i, err
}

func (t TSize) String() string {
	return "size"
}

func (t TFileMode) Convert(s string) (any, error) {
	return t.convert(s)
}

func (t TFileMode) convert(s string) (*os.FileMode, error) {
	var c int
	if s == "" {
		return nil, nil
	}
	switch len(s) {
	case 4:
		var err error
		if c, err = strconv.Atoi(string(s[0])); err != nil {
			return nil, fmt.Errorf("invalid X... digit in %s: must be integer", s)
		}
		s = s[1:]
	case 3:
		c = 0
	default:
		return nil, fmt.Errorf("invalid unix mode %s: must be 3 or 4 digit long", s)
	}
	i, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return nil, err
	}
	mode := os.FileMode(i)
	switch c {
	case 0:
	case 1:
		mode = mode | os.ModeSticky
	case 2:
		mode = mode | os.ModeSetuid
	case 3:
		mode = mode | os.ModeSetuid | os.ModeSticky
	case 4:
		mode = mode | os.ModeSetgid
	case 5:
		mode = mode | os.ModeSetgid | os.ModeSticky
	case 6:
		mode = mode | os.ModeSetgid | os.ModeSticky
	case 7:
		mode = mode | os.ModeSetuid | os.ModeSetgid | os.ModeSticky
	default:
		return nil, fmt.Errorf("invalid X... digit in %s: must be 0-7", s)
	}
	return &mode, nil
}

func (t TFileMode) String() string {
	return "filemode"
}

func ReadFile(fs embed.FS, s string) string {
	if b, err := fs.ReadFile(s); err != nil {
		panic("missing documentation text file: " + s)
	} else {
		return string(b)
	}
}
