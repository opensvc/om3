package check

import (
	"fmt"
	"strconv"

	"github.com/opensvc/om3/v3/util/sizeconv"
)

type (
	// Line is a check result as a listing shows it: the result, its value
	// and its unit kept apart, and the value as a reader reads it, unit
	// included.
	Line struct {
		Result
		ValueText string `json:"value_text"`
	}
)

const (
	// DefaultOutput is the listing of the check results.
	DefaultOutput = "tab=TYPE:type,DRIVER:driver,INSTANCE:instance,OBJECT:path,VALUE:value_text"

	// DefaultSort orders the results by type, the instances of a type
	// together.
	DefaultSort = "TYPE,INSTANCE"
)

// Lines returns the results as a listing shows them.
func (t ResultSet) Lines() []Line {
	l := make([]Line, len(t.Data))
	for i, r := range t.Data {
		l[i] = Line{Result: r, ValueText: ValueText(r.Value, r.Unit)}
	}
	return l
}

// ValueText returns the value with its unit, as a reader reads it: sizes in
// binary units as the other listings show them, speeds in Mb/s or Gb/s,
// and a value of a unit not known followed by its unit.
func ValueText(value int64, unit string) string {
	switch unit {
	case "":
		return strconv.FormatInt(value, 10)
	case "%":
		return fmt.Sprintf("%d%%", value)
	case "kb":
		return sizeconv.BSize(float64(value) * 1024)
	case "Mb/s":
		if value >= 1000 && value%1000 == 0 {
			return fmt.Sprintf("%dGb/s", value/1000)
		}
		return fmt.Sprintf("%dMb/s", value)
	case "C":
		return fmt.Sprintf("%d°C", value)
	default:
		return fmt.Sprintf("%d %s", value, unit)
	}
}
