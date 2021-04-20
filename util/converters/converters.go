package converters

import (
	"errors"
	"strconv"
	"strings"

	"github.com/anmitsu/go-shlex"
	"github.com/golang-collections/collections/set"
)

type (
	NumType   string
	ListType  string
	ShlexType string
)

var (
	Num   NumType
	List  ListType
	Shlex ShlexType

	ErrMissConverter = errors.New("conversion not implemented")
)

func (t NumType) ToBool(s string) (bool, error) {
	return strconv.ParseBool(s)
}

func (t NumType) ToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

func (t NumType) ToInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (t NumType) ToFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func (t NumType) ToSlice(s string) ([]string, error) {
	return []string{}, ErrMissConverter
}

func (t NumType) ToSet(s string) (*set.Set, error) {
	set := set.New()
	for _, e := range strings.Fields(s) {
		set.Insert(e)
	}
	return set, nil
}

func (t ListType) ToBool(s string) (bool, error) {
	return false, ErrMissConverter
}

func (t ListType) ToInt(s string) (int, error) {
	return 0, ErrMissConverter
}

func (t ListType) ToInt64(s string) (int64, error) {
	return 0, ErrMissConverter
}

func (t ListType) ToFloat(s string) (float64, error) {
	return 0.0, ErrMissConverter
}

func (t ListType) ToSlice(s string) ([]string, error) {
	return strings.Fields(s), nil
}

func (t ListType) ToSet(s string) (*set.Set, error) {
	set := set.New()
	for _, e := range strings.Fields(s) {
		set.Insert(e)
	}
	return set, nil
}

func (t ShlexType) ToBool(s string) (bool, error) {
	return false, ErrMissConverter
}

func (t ShlexType) ToInt(s string) (int, error) {
	return 0, ErrMissConverter
}

func (t ShlexType) ToInt64(s string) (int64, error) {
	return 0, ErrMissConverter
}

func (t ShlexType) ToFloat(s string) (float64, error) {
	return 0.0, ErrMissConverter
}

func (t ShlexType) ToSlice(s string) ([]string, error) {
	return shlex.Split(s, true)
}

func (t ShlexType) ToSet(s string) (*set.Set, error) {
	return nil, ErrMissConverter
}
