package converters

import (
	"os/user"
	"strconv"
)

type (
	TUser  struct{}
	TGroup struct{}
)

func init() {
	Register(TUser{})
	Register(TGroup{})
}

func (t TUser) Convert(s string) (interface{}, error) {
	return t.convert(s)
}

func (t TUser) convert(s string) (*user.User, error) {
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.Atoi(s); err == nil {
		return user.LookupId(s)
	}
	return user.Lookup(s)
}

func (t TUser) String() string {
	return "user"
}

func (t TGroup) Convert(s string) (interface{}, error) {
	return t.convert(s)
}

func (t TGroup) convert(s string) (*user.Group, error) {
	if s == "" {
		return nil, nil
	}
	if _, err := strconv.Atoi(s); err == nil {
		return user.LookupGroupId(s)
	}
	return user.LookupGroup(s)
}

func (t TGroup) String() string {
	return "group"
}
