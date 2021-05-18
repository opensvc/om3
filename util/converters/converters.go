package converters

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/anmitsu/go-shlex"
	"github.com/golang-collections/collections/set"
)

type (
	// T is the integer identifier of a converter
	T int
	F func(string) (interface{}, error)
)

const (
	String T = iota
	Int
	Int64
	Float64
	Bool
	List
	ListLowercase
	Set
	Shlex
	Duration
	Umask
)

var (
	toString = map[T]string{
		String:        "string",
		Int:           "int",
		Int64:         "int64",
		Float64:       "float64",
		Bool:          "bool",
		List:          "list",
		ListLowercase: "list-lowercase",
		Set:           "set",
		Shlex:         "Shlex",
		Duration:      "Duration",
		Umask:         "Umask",
	}
	toID = map[string]T{
		"string":         String,
		"int":            Int,
		"int64":          Int64,
		"float64":        Float64,
		"bool":           Bool,
		"list":           List,
		"list-lowercase": ListLowercase,
		"set":            Set,
		"Shlex":          Shlex,
		"Duration":       Duration,
		"Umask":          Umask,
	}
	ErrMissConverter = errors.New("conversion not implemented")
)

func ToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func ToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func ToFloat64(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func ToBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

func ToList(s string) ([]string, error) {
	return strings.Fields(s), nil
}

func ToListLowercase(s string) ([]string, error) {
	l := strings.Fields(s)
	for i := 0; i < len(l); i++ {
		l[i] = strings.ToLower(l[i])
	}
	return l, nil
}

func ToSet(s string) (*set.Set, error) {
	aSet := set.New()
	for _, e := range strings.Fields(s) {
		aSet.Insert(e)
	}
	return aSet, nil
}

func ToShlex(s string) ([]string, error) {
	return shlex.Split(s, true)
}

// ToDuration convert duration string to *time.Duration
//
// nil is returned when duration is unset
// Default unit is second when not specified
func ToDuration(s string) (*time.Duration, error) {
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.Atoi(s); err == nil {
		s = s + "s"
	}
	duration, err := time.ParseDuration(s)
	if err != nil {
		return nil, err
	}
	return &duration, nil
}

func ToUmask(s string) (*os.FileMode, error) {
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

func Convert(s string, t T) (interface{}, error) {
	switch t {
	case String:
		return s, nil
	case Int:
		return ToInt(s)
	case Int64:
		return ToInt64(s)
	case Float64:
		return ToFloat64(s)
	case Bool:
		return ToBool(s)
	case List:
		return ToList(s)
	case ListLowercase:
		return ToListLowercase(s)
	case Set:
		return ToSet(s)
	case Shlex:
		return ToShlex(s)
	case Duration:
		return ToDuration(s)
	case Umask:
		return ToUmask(s)
	default:
		return nil, fmt.Errorf("unknown converter id %d", t)
	}
}
