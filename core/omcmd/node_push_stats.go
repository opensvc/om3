package omcmd

import (
	"context"
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/core/object"
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

// pushStatsTimeLayouts are the forms --begin and --end accept, the local time
// zone applying to those without one.
var pushStatsTimeLayouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02",
}

func parsePushStatsTime(option, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range pushStatsTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("--%s %q: expected RFC 3339 or YYYY-MM-DD[ HH:MM[:SS]]", option, s)
}

func (t *CmdNodePushStats) Run() error {
	begin, err := parsePushStatsTime("begin", t.Begin)
	if err != nil {
		return err
	}
	end, err := parsePushStatsTime("end", t.End)
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
