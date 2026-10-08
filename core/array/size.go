package array

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type (
	// Size is the value of a --size option of an array command: a size in
	// bytes, or a growth in bytes when Relative.
	Size struct {
		Bytes    int64
		Relative bool
	}
)

var (
	// ErrShrink is the error of a resize to a size below the current one,
	// which the arrays do by dropping the end of the volume.
	ErrShrink = errors.New("the new size is below the current size")

	// sizeUnits are the powers of 1024 of the units a size is given in.
	sizeUnits = map[string]int{
		"":  0,
		"k": 1,
		"m": 2,
		"g": 3,
		"t": 4,
		"p": 5,
		"e": 6,
	}
)

// ParseSize returns the size a --size option value says: "1073741824" is
// bytes, "10g" is 10 GiB, and "+1g" grows the volume by 1 GiB.
//
// Every unit is a power of 1024, whether written "g", "gb", "gib", "G",
// "GB" or "GiB". That is how v2 and the collector read "g" and "gb", the
// forms its disk forms take, so a disk is allocated the size the collector
// checked the quota for. v2 read "gib" as a power of 1000, which is not
// kept: a power of 1024 is what the unit means.
//
// A size below zero, as "-1g", is refused: a volume is shrunk by giving its
// new size, which a resize checks against the current one.
func ParseSize(s string) (Size, error) {
	var size Size
	v := strings.TrimSpace(s)
	switch {
	case v == "":
		return size, fmt.Errorf("empty size")
	case strings.HasPrefix(v, "-"):
		return size, fmt.Errorf("size %q: a size can not be negative: give the new size to shrink", s)
	case strings.HasPrefix(v, "+"):
		size.Relative = true
		v = strings.TrimSpace(v[1:])
	}
	i := strings.IndexFunc(v, func(r rune) bool {
		return (r < '0' || r > '9') && r != '.'
	})
	number, unit := v, ""
	if i >= 0 {
		number, unit = v[:i], strings.ToLower(strings.TrimSpace(v[i:]))
	}
	if number == "" {
		return size, fmt.Errorf("size %q: no number", s)
	}
	unit = strings.TrimSuffix(unit, "b")
	unit = strings.TrimSuffix(unit, "i")
	power, ok := sizeUnits[unit]
	if !ok {
		return size, fmt.Errorf("size %q: unknown unit", s)
	}
	f, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return size, fmt.Errorf("size %q: %w", s, err)
	}
	f *= math.Pow(1024, float64(power))
	if f >= math.MaxInt64 {
		return size, fmt.Errorf("size %q: too large", s)
	}
	size.Bytes = int64(f)
	if size.Bytes == 0 && !size.Relative {
		return size, fmt.Errorf("size %q: a volume can not be empty", s)
	}
	return size, nil
}

// Target returns the size in bytes a volume of current bytes is resized to.
func (t Size) Target(current int64) int64 {
	if t.Relative {
		return current + t.Bytes
	}
	return t.Bytes
}

// CheckResize returns an error when resizing a volume of current bytes to
// target bytes drops data, which is a shrink, unless truncate allows it.
//
// The arrays shrink a volume by dropping its end, where the filesystem or the
// database on it may hold data, so a shrink is done only when asked for by
// name, with --truncate.
func CheckResize(current, target int64, truncate bool) error {
	if target < current && !truncate {
		return fmt.Errorf("%w: %d bytes to %d bytes, which drops the end of the volume: --truncate allows it", ErrShrink, current, target)
	}
	return nil
}
