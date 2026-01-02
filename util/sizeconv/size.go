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
	dAbb = []string{"", "K", "M", "G", "T", "P", "E", "Z", "Y"}
	bAbb = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	sReg = regexp.MustCompile(`^(\d+([\.,]\d+)*) ?([kKmMgGtTpPeE])?([iI])?([bB])?$`)
)

func getSizeAndUnit(size float64, base float64, _map []string, exact, compact bool) (float64, string) {
	i := 0
	unitsLimit := len(_map) - 1
	for size >= base && i < unitsLimit {
		if exact && math.Mod(size, base) != 0 {
			break
		}
		size = size / base
		i++
	}
	unit := _map[i]
	if compact {
		unit = strings.ToLower(unit)
	}
	return size, unit
}

// PrintSigFixed formats a float to N significant digits using fixed-point notation.
func PrintSigFixed(value float64, N int, compact bool) string {
	if value == 0 {
		if compact {
			return "0"
		}
		// Handle zero case to avoid log(0)
		return fmt.Sprintf("0.%s", strings.Repeat("0", N-1))
	}

	// Calculate the number of digits before the decimal point (e.g., 123.45 has 3)
	// math.Log10(123.45) is approx 2.09. math.Floor(2.09) is 2. 2 + 1 = 3 digits.
	// For small numbers like 0.00123, log10 is -2.9. math.Floor(-2.9) is -3. -3 + 1 = -2.
	digitsBeforeDecimal := int(math.Floor(math.Log10(math.Abs(value)))) + 1

	// Calculate the required decimal precision for %f
	// Decimal places (D) = N (significant digits) - digitsBeforeDecimal
	decimalPlaces := N - digitsBeforeDecimal

	if decimalPlaces < 0 {
		// If the number is large (e.g., 12345 and N=3), 3 - 5 = -2.
		// Set precision to 0, and the formatting will handle rounding to the nearest power of 10.
		decimalPlaces = 0
	}

	// Use %f with the calculated decimal precision
	format := fmt.Sprintf("%%.%df", decimalPlaces)

	s := fmt.Sprintf(format, value)
	if strings.Contains(s, ".") && compact {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}

// CustomSize returns a human-readable approximation of a size
// using custom format and precision.
func CustomSize(format string, precision int, size float64, base float64, _map []string, compact bool) string {
	size, unit := getSizeAndUnit(size, base, _map, false, compact)
	return PrintSigFixed(size, precision, compact) + unit
}

func CustomExactSize(format string, precision int, size float64, base float64, _map []string, compact bool) string {
	size, unit := getSizeAndUnit(size, base, _map, true, compact)
	return fmt.Sprintf(format, precision, size, unit)
}

// DSizeWithPrecision returns a human-readable, arbitrary precision,
// representation of size in SI units.
func DSizeWithPrecision(size float64, precision int) string {
	return CustomSize("%.*g%s", precision, size, 1000.0, dAbb, false)
}

// DSize returns a human-readable, default precision,
// representation of size in SI units.
func DSize(size float64) string {
	return CustomSize("%.*g%s", defaultPrecision, size, 1000.0, dAbb, false)
}

func DSizeCompact(f float64) string {
	return CustomSize("%.*g%s", defaultPrecision, f, 1000.0, dAbb, true)
}

// BSizeWithPrecision returns a human-readable, arbitrary precision,
// representation of size in binary units.
func BSizeWithPrecision(size float64, precision int) string {
	return CustomSize("%.*g%s", precision, size, 1024.0, bAbb, false)
}

// BSize returns a human-readable, default precision,
// representation of size in binary units.
func BSize(size float64) string {
	return CustomSize("%.*g%s", defaultPrecision, size, 1024.0, bAbb, false)
}

// BSizeCompact returns a compact human readable version of n
func BSizeCompact(f float64) string {
	return CustomSize("%.*g%s", defaultPrecision, f, 1024.0, bAbb, true)
}

func ExactBSizeCompact(f float64) string {
	size, unit := getSizeAndUnit(f, 1024.0, bAbb, true, true)
	return fmt.Sprintf("%.0f%s", size, unit)
}

func ExactDSizeCompact(f float64) string {
	size, unit := getSizeAndUnit(f, 1000.0, bAbb, true, true)
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
	} else if strings.ToLower(matches[5]) == "b" {
		convertMap = dMap
	} else if matches[5] == "" {
		convertMap = bMap
	} else {
		return -1, fmt.Errorf("invalid size unit: '%s'", sizeStr)
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
