package sizeconv

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	// KB is KiloBytes
	KB = 1000
	// MB is MegaBytes
	MB = 1000 * KB
	// GB is GigaBytes
	GB = 1000 * MB
	// TB is TeraBytes
	TB = 1000 * GB
	// PB is PetaBytes
	PB = 1000 * TB

	// KiB is KibiBytes
	KiB = 1024
	// MiB is MibiBytes
	MiB = 1024 * KiB
	// GiB is GibiBytes
	GiB = 1024 * MiB
	// TiB is TibiBytes
	TiB = 1024 * GiB
	// PiB is PibiBytes
	PiB = 1024 * TiB

	defaultPrecision = 3
)

type unitMap map[string]int64

var (
	dMap = unitMap{"k": KB, "m": MB, "g": GB, "t": TB, "p": PB}
	bMap = unitMap{"k": KiB, "m": MiB, "g": GiB, "t": TiB, "p": PiB}
	dAbb = []string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	bAbb = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	cAbb = []string{"", "b", "m", "g", "t", "p", "e", "z", "y"}
	sReg = regexp.MustCompile(`^(\d+(\.\d+)*) ?([kKmMgGtTpP])?[iI]?[bB]?$`)
)

func getSizeAndUnit(size float64, base float64, _map []string) (float64, string) {
	i := 0
	unitsLimit := len(_map) - 1
	for size >= base && i < unitsLimit {
		size = size / base
		i++
	}
	return size, _map[i]
}

// CustomSize returns a human-readable approximation of a size
// using custom format and precision.
func CustomSize(format string, precision int, size float64, base float64, _map []string) string {
	size, unit := getSizeAndUnit(size, base, _map)
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
	return CustomSize("%.4g%s", defaultPrecision, size, 1024.0, bAbb)
}

// FromDSize returns an integer from a human-readable representation of a
// size using SI standard (eg. "44kB", "17MB").
func FromDSize(size string) (int64, error) {
	return parseSize(size, dMap)
}

// Parses the human-readable size string into a bytes count.
func parseSize(sizeStr string, uMap unitMap) (int64, error) {
	matches := sReg.FindStringSubmatch(sizeStr)
	if len(matches) != 4 {
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
	s := CustomSize("%.*g%s", defaultPrecision, f, 1024.0, cAbb)
	//s = strings.ReplaceAll(s, " ", "")
	//return strings.ToLower(s)
	return s
}
