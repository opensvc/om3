package sizeconv

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	// KB is KiloByte
	KB = 1000
	// MB is MegaByte
	MB = 1000 * KB
	// GB is GigaByte
	GB = 1000 * MB
	// TB is TeraByte
	TB = 1000 * GB
	// PB is PetaByte
	PB = 1000 * TB
	// EB is ExaByte
	EB = 1000 * PB

	// KiB is KibiByte
	KiB = 1024
	// MiB is MibiByte
	MiB = 1024 * KiB
	// GiB is GibiByte
	GiB = 1024 * MiB
	// TiB is TibiByte
	TiB = 1024 * GiB
	// PiB is Pebibyte
	PiB = 1024 * TiB
	// EiB is Exbibyte
	EiB = 1024 * PiB

	defaultPrecision = 3
)

type unitMap map[string]int64

var (
	dMap = unitMap{"k": KB, "m": MB, "g": GB, "t": TB, "p": PB, "e": EB}
	bMap = unitMap{"k": KiB, "m": MiB, "g": GiB, "t": TiB, "p": PiB, "e": EiB}
	dAbb = []string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	bAbb = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	cAbb = []string{"", "k", "m", "g", "t", "p", "e", "z", "y"}
	sReg = regexp.MustCompile(`^(\d+([\.,]\d+)*) ?([kKmMgGtTpPeE])?([iI])?([bB])?$`)
)

func getSizeAndUnit(size float64, base float64, _map []string, exact bool) (float64, string) {
	i := 0
	unitsLimit := len(_map) - 1
	for size >= base && i < unitsLimit {
		if exact && math.Mod(size, base) != 0 {
			break
		}
		size = size / base
		i++
	}
	return size, _map[i]
}

// CustomSize returns a human-readable approximation of a size
// using custom format and precision.
func CustomSize(format string, precision int, size float64, base float64, _map []string) string {
	size, unit := getSizeAndUnit(size, base, _map, false)
	return fmt.Sprintf(format, precision, size, unit)
}

func CustomExactSize(format string, precision int, size float64, base float64, _map []string) string {
	size, unit := getSizeAndUnit(size, base, _map, true)
	return fmt.Sprintf(format, precision, size, unit)
}

// DSizeWithPrecision returns a human-readable, arbitrary precision,
// representation of size in SI units.
func DSizeWithPrecision(size float64, precision int) string {
	return CustomSize("%.*g%s", precision, size, 1000.0, dAbb)
}

// DSize returns a human-readable, default precision,
// representation of size in SI units.
func DSize(size float64) string {
	return CustomSize("%.*g%s", defaultPrecision, size, 1000.0, dAbb)
}

// BSizeWithPrecision returns a human-readable, arbitrary precision,
// representation of size in binary units.
func BSizeWithPrecision(size float64, precision int) string {
	return CustomSize("%.*g%s", precision, size, 1024.0, bAbb)
}

// BSize returns a human-readable, default precision,
// representation of size in binary units.
func BSize(size float64) string {
	return CustomSize("%.*g%s", defaultPrecision, size, 1024.0, bAbb)
}

// BSizeCompact returns a compact human readable version of n
func BSizeCompact(f float64) string {
	return CustomSize("%.*g%s", defaultPrecision, f, 1024.0, cAbb)
}

func ExactBSizeCompact(f float64) string {
	size, unit := getSizeAndUnit(f, 1024.0, cAbb, true)
	return fmt.Sprintf("%.0f%s", size, unit)
}

func ExactDSizeCompact(f float64) string {
	size, unit := getSizeAndUnit(f, 1000.0, cAbb, true)
	return fmt.Sprintf("%.0f%s", size, unit)
}

// FromSize returns an integer from a human-readable representation of a
// size using Metric and IEC standard (eg. "44KiB", "17MiB", "20MB", "7.5EiB").
// Max possible value is MaxInt64 (< 8EiB)
func FromSize(sizeStr string) (int64, error) {
	matches := sReg.FindStringSubmatch(sizeStr)
	if len(matches) != 6 {
		return -1, fmt.Errorf("invalid size: '%s'", sizeStr)
	}

	var convertMap unitMap
	if strings.ToLower(matches[4]) == "i" {
		convertMap = bMap
	} else if strings.ToLower(matches[5]) == "" {
		// eg. "100m" interpreted implicitly as "100MiB"
		convertMap = bMap
	} else {
		convertMap = dMap
	}
	dotted := strings.ReplaceAll(matches[1], ",", ".")
	size, err := strconv.ParseFloat(dotted, 64)
	if err != nil {
		return -1, err
	}

	unitPrefix := strings.ToLower(matches[3])

	if mul, ok := convertMap[unitPrefix]; ok {
		size *= float64(mul)
	}
	if size > math.MaxInt64 || int64(size) < 0 {
		return -1, fmt.Errorf("max size for int64: '%s'", sizeStr)
	}
	return int64(size), nil
}

// FromDSize returns an integer from a human-readable representation of a
// size using SI standard (eg. "44kB", "17MB").
func FromDSize(size string) (int64, error) {
	return parseSize(size, dMap)
}

// Parses the human-readable size string into a bytes count.
func parseSize(sizeStr string, uMap unitMap) (int64, error) {
	matches := sReg.FindStringSubmatch(sizeStr)
	if len(matches) != 6 {
		return -1, fmt.Errorf("invalid size: '%s'", sizeStr)
	}

	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return -1, err
	}

	unitPrefix := strings.ToLower(matches[3])
	if mul, ok := uMap[unitPrefix]; ok {
		size *= float64(mul)
	}

	return int64(size), nil
}

// BSizeCompactFromMB returns a compact human readable version of n
func BSizeCompactFromMB(n uint64) string {
	f := float64(n * MiB)
	return BSizeCompact(f)
}
