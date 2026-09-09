package array

import (
	"context"
	"fmt"
)

type (
	// Report is one section of the configuration of an array, as the
	// collector expects it.
	//
	// An array is reported as a list of section names and a list of the
	// contents of those sections, side by side. The collector reads the names
	// to know what it was handed, so they are the names v2 sent: a collector
	// serving both agents stores what either of them pushes in the same
	// place.
	Report struct {
		// Key is the name of the section.
		Key string

		// Get returns the content of the section, as the array answered it.
		Get func(ctx context.Context) (any, error)
	}

	// Reporter is implemented by an array driver whose configuration can be
	// pushed to the collector.
	Reporter interface {
		Reports() []Report
	}
)

// ReportData is what an array pushes: the names of its sections, and their
// contents in the same order.
type ReportData struct {
	Keys   []string `json:"keys"`
	Values []any    `json:"values"`
}

// Collect asks an array for every section it reports.
//
// A section the array refuses to answer stops the collection: half a
// configuration reported as a whole one would have the collector delete what
// it thinks has gone away.
func Collect(ctx context.Context, reporter Reporter) (ReportData, error) {
	var data ReportData
	for _, report := range reporter.Reports() {
		value, err := report.Get(ctx)
		if err != nil {
			return data, fmt.Errorf("%s: %w", report.Key, err)
		}
		data.Keys = append(data.Keys, report.Key)
		data.Values = append(data.Values, value)
	}
	return data, nil
}

// ReportMethod returns the name of the collector method an array of this type
// is pushed with.
//
// The collector exports one method per array type, named as v2 named it, so
// this is a contract with the collector rather than a choice.
func ReportMethod(arrayType string) string {
	return "update_" + arrayType
}
