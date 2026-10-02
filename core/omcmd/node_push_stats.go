package omcmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/duration"
)

type (
	CmdNodePushStats struct {
		OptsGlobal
		Begin                       string
		End                         string
		StatsDir                    string
		DryRun                      bool
		IgnoreNoCollectorConfigured bool
	}
)

// pushStatsTimeLayouts are the dated forms --begin and --end accept, the
// local time zone applying to those without one.
var pushStatsTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

// pushStatsClockLayouts are the forms of a time of the current day.
var pushStatsClockLayouts = []string{
	"15:04:05",
	"15:04",
}

// parsePushStatsTime reads a --begin or --end value: a time ago, as -1d2h or
// -30m, a date, RFC 3339 or YYYY-MM-DD[ HH:MM[:SS]], or a time of today,
// HH:MM[:SS]. The local time zone applies to the forms without one.
func parsePushStatsTime(option, s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if strings.HasPrefix(s, "-") {
		d, err := duration.Parse(s)
		if err != nil || d >= 0 {
			return time.Time{}, fmt.Errorf("--%s %q: a time ago is a negative duration, as -1d2h or -30m", option, s)
		}
		return now.Add(d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range pushStatsTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	for _, layout := range pushStatsClockLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			local := now.In(time.Local)
			return time.Date(local.Year(), local.Month(), local.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local), nil
		}
	}
	return time.Time{}, fmt.Errorf("--%s %q: expected a time ago, as -1d2h, RFC 3339, YYYY-MM-DD[ HH:MM[:SS]], or HH:MM[:SS] for today", option, s)
}

func (t *CmdNodePushStats) Run() error {
	now := time.Now()
	begin, err := parsePushStatsTime("begin", t.Begin, now)
	if err != nil {
		return err
	}
	end, err := parsePushStatsTime("end", t.End, now)
	if err != nil {
		return err
	}
	options := object.PushStatsOptions{Begin: begin, End: end, StatsDir: t.StatsDir}
	err = nodeaction.New(
		// The statistics are read from the local sysstat files.
		nodeaction.WithLocal(true),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
		nodeaction.WithLocalFunc(func() (interface{}, error) {
			n, err := object.NewNode()
			if err != nil {
				return nil, err
			}
			if t.DryRun {
				return n.PushStatsDryRun(context.Background(), options)
			}
			return n.PushStats(context.Background(), options)
		}),
	).Do()

	if err != nil && t.IgnoreNoCollectorConfigured && isNoCollectorError(err) {
		return nil
	}
	return err
}
