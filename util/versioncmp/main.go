// Package versioncmp compares the versions of the tools the drivers depend
// on, as "lxc-info --version" or "virsh --version" print them.
//
// Those are no semantic versions: a distribution appends its own suffix, as
// "5.0.0~git2209-g5a7b9ce67" or "4.0.12-0ubuntu1~20.04.1", so only the
// numeric components are compared.
package versioncmp

import (
	"fmt"
	"strconv"
	"strings"
)

// Parse returns the numeric components of a version: the dot separated
// numbers it starts with, after an optional "v", up to the first character
// that is neither a digit nor a dot. "5.0.0~git2209" is [5 0 0].
func Parse(s string) ([]int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	end := strings.IndexFunc(s, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	if end >= 0 {
		s = s[:end]
	}
	s = strings.TrimRight(s, ".")
	if s == "" {
		return nil, fmt.Errorf("no version number")
	}
	l := make([]int, 0, 3)
	for _, part := range strings.Split(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("version number %s: %w", s, err)
		}
		l = append(l, n)
	}
	return l, nil
}

// Compare returns -1, 0 or 1 as the version a is older than, the same as, or
// newer than the version b. A component one has and the other has not is
// compared to 0, so 2.1 and 2.1.0 are the same.
func Compare(a, b string) (int, error) {
	va, err := Parse(a)
	if err != nil {
		return 0, fmt.Errorf("%q: %w", a, err)
	}
	vb, err := Parse(b)
	if err != nil {
		return 0, fmt.Errorf("%q: %w", b, err)
	}
	for i := 0; i < max(len(va), len(vb)); i++ {
		var na, nb int
		if i < len(va) {
			na = va[i]
		}
		if i < len(vb) {
			nb = vb[i]
		}
		switch {
		case na < nb:
			return -1, nil
		case na > nb:
			return 1, nil
		}
	}
	return 0, nil
}

// AtLeast says whether the version v is min or newer.
func AtLeast(v, min string) (bool, error) {
	n, err := Compare(v, min)
	return n >= 0, err
}

// Newer says whether the version v is newer than than.
func Newer(v, than string) (bool, error) {
	n, err := Compare(v, than)
	return n > 0, err
}
