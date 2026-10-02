package sizeconv

import (
	"fmt"
	"strconv"
	"strings"
)

// Share is a part of a whole, written as a percentage of it, "10" or "10%",
// or as a size, "512m" or "2Gi".
//
// A number with no unit is a percentage: a share of memory written as a
// count of bytes is no share anyone means.
type Share struct {
	// Percent is the share as a percentage, when it is written as one.
	Percent int

	// Size is the share as a size in bytes, when it is written as one.
	Size int64

	// IsSize says the share is written as a size.
	IsSize bool
}

// ParseShare parses a share written as a percentage, "10" or "10%", or as a
// size, "512m", "2g" or "2Gi".
func ParseShare(s string) (Share, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Share{}, fmt.Errorf("empty share")
	}
	if digits, isPercent := strings.CutSuffix(s, "%"); isPercent || isDigits(s) {
		pct, err := strconv.Atoi(strings.TrimSpace(digits))
		if err != nil || pct < 0 || pct > 100 {
			return Share{}, fmt.Errorf("invalid share %q: a percentage is a whole number from 0 to 100", s)
		}
		return Share{Percent: pct}, nil
	}
	size, err := FromSize(s)
	if err != nil {
		return Share{}, fmt.Errorf("invalid share %q: neither a percentage, as 10%%, nor a size, as 2Gi", s)
	}
	return Share{Size: size, IsSize: true}, nil
}

// PercentOf returns the share as a whole percentage of total, in bytes.
//
// A size is truncated to the percentage it amounts to, and bounded by
// limit: a size beyond what is reasonable for the whole it is a share of is
// read as that limit. An unknown total, zero, has no share: there is
// nothing to measure against.
func (t Share) PercentOf(total int64, limit int) int {
	if total <= 0 {
		return 0
	}
	if !t.IsSize {
		return t.Percent
	}
	pct := int(t.Size * 100 / total)
	if t.Size > total {
		pct = 100
	}
	return min(pct, limit)
}

// String returns the share as it is written.
func (t Share) String() string {
	if t.IsSize {
		return ExactBSizeCompact(float64(t.Size))
	}
	return strconv.Itoa(t.Percent) + "%"
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
