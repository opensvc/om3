package sizeconv

import (
	"fmt"
	"strings"
)

// Change is a size a user asked for, which is either the size to reach or the
// amount to add or remove.
type Change struct {
	// Value is the size, or the amount, in bytes.
	Value int64

	// IsRelative says Value is an amount to add to the current size, or to
	// remove from it when negative.
	IsRelative bool
}

// ParseChange reads the size a user asked for.
//
// A leading "+" or "-" makes it relative to the current size, so "+1g" grows
// by a gigabyte where "11g" grows or shrinks to eleven. The rest is read the
// way every other size om3 accepts is read: "11g" and "12Gi" are binary,
// "11GB" is decimal.
func ParseChange(s string) (Change, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Change{}, fmt.Errorf("empty size")
	}
	sign := int64(1)
	relative := false
	switch s[0] {
	case '+':
		relative, s = true, s[1:]
	case '-':
		relative, sign, s = true, -1, s[1:]
	}
	if s == "" {
		return Change{}, fmt.Errorf("size with no value")
	}
	value, err := FromSize(s)
	if err != nil {
		return Change{}, err
	}
	return Change{Value: sign * value, IsRelative: relative}, nil
}

// Resolve is the size to reach, given the size held now.
func (t Change) Resolve(current int64) int64 {
	if t.IsRelative {
		return current + t.Value
	}
	return t.Value
}

func (t Change) String() string {
	if !t.IsRelative {
		return BSizeCompact(float64(t.Value))
	}
	if t.Value < 0 {
		return "-" + BSizeCompact(float64(-t.Value))
	}
	return "+" + BSizeCompact(float64(t.Value))
}
