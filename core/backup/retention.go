package backup

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type (
	// Retention is the runs of an object a keep keeps: the latest ones,
	// and the latest one of each of the latest periods.
	Retention struct {
		// Last is the number of latest runs kept.
		Last int

		// Periods are the rules keeping the latest run of each of the
		// latest periods of a unit.
		Periods []Period
	}

	// Period keeps the latest run of each of the Count latest periods of
	// Unit holding a run.
	Period struct {
		Unit  PeriodUnit
		Count int
	}

	// PeriodUnit is the unit of a period: h, d, w, m or y.
	PeriodUnit byte
)

const (
	Hour  PeriodUnit = 'h'
	Day   PeriodUnit = 'd'
	Week  PeriodUnit = 'w'
	Month PeriodUnit = 'm'
	Year  PeriodUnit = 'y'
)

// ParseRetention parses a keep: rules separated by spaces, each a number of
// latest runs, as 10, or a number of periods, as 24h, 7d, 4w, 6m or 1y.
func ParseRetention(s string) (Retention, error) {
	var r Retention
	for _, word := range strings.Fields(s) {
		if n, err := strconv.Atoi(word); err == nil {
			if n < 1 {
				return Retention{}, fmt.Errorf("keep %q: %q keeps no run", s, word)
			}
			r.Last = max(r.Last, n)
			continue
		}
		unit := PeriodUnit(word[len(word)-1])
		switch unit {
		case Hour, Day, Week, Month, Year:
		default:
			return Retention{}, fmt.Errorf("keep %q: %q is neither a number of runs nor a number of periods (h, d, w, m or y)", s, word)
		}
		n, err := strconv.Atoi(word[:len(word)-1])
		if err != nil || n < 1 {
			return Retention{}, fmt.Errorf("keep %q: %q is neither a number of runs nor a number of periods (h, d, w, m or y)", s, word)
		}
		if i := slices.IndexFunc(r.Periods, func(p Period) bool { return p.Unit == unit }); i >= 0 {
			r.Periods[i].Count = max(r.Periods[i].Count, n)
			continue
		}
		r.Periods = append(r.Periods, Period{Unit: unit, Count: n})
	}
	return r, nil
}

// IsZero tells whether the retention keeps every run: no rule, as no keep.
func (r Retention) IsZero() bool {
	return r.Last == 0 && len(r.Periods) == 0
}

// String returns the retention as a keep says it.
func (r Retention) String() string {
	l := make([]string, 0, len(r.Periods)+1)
	if r.Last > 0 {
		l = append(l, strconv.Itoa(r.Last))
	}
	for _, p := range r.Periods {
		l = append(l, fmt.Sprintf("%d%c", p.Count, p.Unit))
	}
	return strings.Join(l, " ")
}

// Drop returns the runs the retention does not keep, oldest first. A run kept
// by any rule is kept.
//
// The periods are counted among the ones holding a run, as restic, borg and
// proxmox count them, rather than back from now: an object whose runs stopped
// for months keeps its last ones instead of losing them all at its next run.
// They are in UTC, of the time a run started, so runs written by different
// nodes fall in the same periods.
func (r Retention) Drop(runs []Run) []Run {
	if r.IsZero() {
		return nil
	}
	newest := slices.Clone(runs)
	slices.SortStableFunc(newest, func(a, b Run) int { return b.CreatedAt.Compare(a.CreatedAt) })
	kept := make(map[int]bool)
	for i := range min(r.Last, len(newest)) {
		kept[i] = true
	}
	for _, p := range r.Periods {
		seen := make(map[string]bool)
		for i, run := range newest {
			if len(seen) == p.Count {
				break
			}
			bucket := p.Unit.bucket(run.CreatedAt)
			if seen[bucket] {
				continue
			}
			seen[bucket] = true
			kept[i] = true
		}
	}
	dropped := make([]Run, 0, len(newest)-len(kept))
	for i := len(newest) - 1; i >= 0; i-- {
		if !kept[i] {
			dropped = append(dropped, newest[i])
		}
	}
	return dropped
}

// bucket returns the period of the unit tm falls in, in UTC.
func (u PeriodUnit) bucket(tm time.Time) string {
	tm = tm.UTC()
	switch u {
	case Hour:
		return tm.Format("2006-01-02T15")
	case Day:
		return tm.Format("2006-01-02")
	case Week:
		year, week := tm.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case Month:
		return tm.Format("2006-01")
	default:
		return tm.Format("2006")
	}
}
