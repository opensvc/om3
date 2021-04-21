package converters

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/golang-collections/collections/set"
)

type (
	// T is the integer identifier of a converter
	T int
)

const (
	Int T = iota
	Int64
	Float64
	Bool
	List
	Set
	Shlex
)

var (
	toString = map[T]string{
		Int:     "int",
		Int64:   "int64",
		Float64: "float64",
		Bool:    "bool",
		List:    "list",
		Set:     "set",
		Shlex:   "Shlex",
	}
	toID = map[string]T{
		"int":     Int,
		"int64":   Int64,
		"float64": Float64,
		"bool":    Bool,
		"list":    List,
		"set":     Set,
		"Shlex":   Shlex,
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

func ToSet(s string) (*set.Set, error) {
	set := set.New()
	for _, e := range strings.Fields(s) {
		set.Insert(e)
	}
	return set, nil
}

func ToShlex(s string) ([]string, error) {
	return shlex.Split(s, true)
}

func Convert(s string, t T) (interface{}, error) {
	switch t {
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
	case Set:
		return ToSet(s)
	case Shlex:
		return ToShlex(s)
	default:
		return nil, fmt.Errorf("unknown converter id %d", t)
	}
}
