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
	TString        string
	TInt           string
	TInt64         string
	TFloat64       string
	TBool          string
	TList          string
	TListLowercase string
	TSet           string
	TShlex         string
	TDuration      string
	TUmask         string
	TSize          string
	TFileMode      string
	TTristate      string
)

var (
	String        TString
	Int           TInt
	Int64         TInt64
	Float64       TFloat64
	Bool          TBool
	List          TList
	ListLowercase TListLowercase
	Set           TSet
	Shlex         TShlex
	Duration      TDuration
	Umask         TUmask
	Size          TSize
	FileMode      TFileMode
	Tristate      TTristate
)

func (t TTristate) Convert(s string) (interface{}, error) {
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

func (t TString) Convert(s string) (interface{}, error) {
	return s, nil
}

func (t TString) String() string {
	return "string"
}

func (t TInt) Convert(s string) (interface{}, error) {
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

func (t TInt64) Convert(s string) (interface{}, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (t TInt64) String() string {
	return "int64"
}

func (t TFloat64) Convert(s string) (interface{}, error) {
	return strconv.ParseFloat(s, 64)
}

func (t TFloat64) String() string {
	return "float64"
}

func (t TBool) Convert(s string) (interface{}, error) {
	if s == "" {
		return false, nil
	}
	s = strings.TrimSpace(s)
	return strconv.ParseBool(s)
}

func (t TBool) String() string {
	return "bool"
}

func (t TList) Convert(s string) (interface{}, error) {
	return strings.Fields(s), nil
}

func (t TList) String() string {
	return "list"
}

func (t TListLowercase) Convert(s string) (interface{}, error) {
	l := strings.Fields(s)
	for i := 0; i < len(l); i++ {
		l[i] = strings.ToLower(l[i])
	}
	return l, nil
}

func (t TListLowercase) String() string {
	return "list-lowercase"
}

func (t TSet) Convert(s string) (interface{}, error) {
	aSet := set.New()
	for _, e := range strings.Fields(s) {
		aSet.Insert(e)
	}
	return aSet, nil
}

func (t TSet) String() string {
	return "set"
}

func (t TShlex) Convert(s string) (interface{}, error) {
	return shlex.Split(s, true)
}

func (t TShlex) String() string {
	return "shlex"
}

// Convert converts duration string to *time.Duration
//
// nil is returned when duration is unset
// Default unit is second when not specified
func (t TDuration) Convert(s string) (interface{}, error) {
	return t.convert(s)
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

func (t TUmask) Convert(s string) (interface{}, error) {
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

func (t TSize) Convert(s string) (interface{}, error) {
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

func (t TFileMode) Convert(s string) (interface{}, error) {
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
	return "file-mode"
}

func ReadFile(fs embed.FS, s string) string {
	if b, err := fs.ReadFile(s); err != nil {
		panic("missing documentation text file: " + s)
	} else {
		return string(b)
	}
}
