package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/util/httphelper"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/nodestats"
)

type (
	// PushStatsOptions bound the statistics a push sends. A zero Begin starts
	// where the last scheduled push left, a zero End is now.
	PushStatsOptions struct {
		Begin    time.Time
		End      time.Time
		StatsDir string
	}

	// PushStatsResult tells what a push read and what the collector stored.
	PushStatsResult struct {
		Begin time.Time `json:"begin"`
		End   time.Time `json:"end"`

		// Batches is the number of requests the range was sent in, a day
		// of statistics each.
		Batches int `json:"batches"`

		// Rows is the number of rows read, by group.
		Rows map[string]int `json:"rows"`

		Series   int      `json:"series"`
		Points   int      `json:"points"`
		Skipped  []string `json:"skipped,omitempty"`
		Disabled []string `json:"disabled,omitempty"`

		// DryRun says the statistics were read and not sent.
		DryRun bool `json:"dry_run,omitempty"`
	}

	oc3NodeStatsBody struct {
		Data nodestats.Stats `json:"data"`
	}

	oc3NodeStatsStored struct {
		Series  int      `json:"series"`
		Points  int      `json:"points"`
		Skipped []string `json:"skipped"`
	}
)

const (
	// pushStatsMinInterval and pushStatsMaxInterval bound the range a push
	// reads when it starts from the last one, as the v2 agent did: at least 21
	// minutes, at most a day and 10 minutes.
	pushStatsMinInterval = 21 * time.Minute
	pushStatsMaxInterval = 1450 * time.Minute

	// pushStatsOverlap is read again before the last push, so that a sample
	// written while it ran is not missed. The collector overwrites the points
	// it already has.
	pushStatsOverlap = 10 * time.Minute

	// pushStatsTimeout is the time the collector is given to store a day of
	// statistics, a file per metric, cpu and device.
	pushStatsTimeout = 2 * time.Minute
)

// pushStatsRange returns the range a push reads: the one asked, or from the last
// scheduled push to now, within the bounds of the v2 agent.
func pushStatsRange(o PushStatsOptions, last, now time.Time) (time.Time, time.Time) {
	end := o.End
	if end.IsZero() {
		end = now
	}
	if !o.Begin.IsZero() {
		return o.Begin, end
	}
	interval := pushStatsMaxInterval
	if !last.IsZero() {
		interval = min(max(end.Sub(last)+pushStatsOverlap, pushStatsMinInterval), pushStatsMaxInterval)
	}
	return end.Add(-interval), end
}

// pushStatsBatches splits a range into the local days it spans, the unit
// sysstat keeps its samples in, a file a day. A batch is read, sent and let go
// before the next is read, so a push of a year holds a day in memory, not the
// year.
//
// A batch ends just before the midnight the next begins at: the samples are
// read between both ends included, and one dated midnight is sent once.
func pushStatsBatches(begin, end time.Time) [][2]time.Time {
	var l [][2]time.Time
	for begin.Before(end) {
		local := begin.In(time.Local)
		midnight := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, time.Local)
		if !midnight.Before(end) {
			l = append(l, [2]time.Time{begin, end})
			break
		}
		l = append(l, [2]time.Time{begin, midnight.Add(-time.Nanosecond)})
		begin = midnight
	}
	return l
}

// pushStatsRun reads the statistics of the range a day at a time, but for the
// groups the stats.disable keyword lists, and hands each day to send. The
// file system usage, which sysstat does not keep, is the one of now, read with
// the last day.
//
// It stops at the first day it can not read or send: the result tells the
// range done, the days before it being stored.
func (t *Node) pushStatsRun(ctx context.Context, o PushStatsOptions, send func(context.Context, nodestats.Stats, *PushStatsResult) error) (PushStatsResult, error) {
	last := t.newScheduleEntry("pushstats", "stats", "", "stats_push").LastRunAt
	begin, end := pushStatsRange(o, last, time.Now())
	disabled := t.MergedConfig().GetStrings(key.New("stats", "disable"))
	result := PushStatsResult{Begin: begin, End: begin, Rows: map[string]int{}, Disabled: disabled}
	if !end.After(begin) {
		return result, fmt.Errorf("the end %s is not after the begin %s", end, begin)
	}
	batches := pushStatsBatches(begin, end)
	for i, batch := range batches {
		disable := disabled
		if i < len(batches)-1 {
			disable = append(slices.Clone(disabled), "fs_u")
		}
		stats, err := nodestats.Collect(ctx, nodestats.Options{Begin: batch[0], End: batch[1], Dir: o.StatsDir, Disable: disable})
		if err != nil {
			return result, fmt.Errorf("read %s to %s: %w", pushStatsTime(batch[0]), pushStatsTime(batch[1]), err)
		}
		if err := send(ctx, stats, &result); err != nil {
			return result, fmt.Errorf("push %s to %s: %w", pushStatsTime(batch[0]), pushStatsTime(batch[1]), err)
		}
		for group, g := range stats {
			result.Rows[group] += len(g.Rows)
		}
		result.Batches++
		result.End = batch[1]
		if len(batches) > 1 {
			t.log.Infof("pushstats: batch %d/%d, %s to %s, done", i+1, len(batches), pushStatsTime(batch[0]), pushStatsTime(batch[1]))
		}
	}
	result.End = end
	return result, nil
}

// PushStatsDryRun reads the statistics a push would send, and sends nothing.
func (t *Node) PushStatsDryRun(ctx context.Context, o PushStatsOptions) (PushStatsResult, error) {
	result, err := t.pushStatsRun(ctx, o, func(context.Context, nodestats.Stats, *PushStatsResult) error { return nil })
	result.DryRun = true
	return result, err
}

// PushStats sends the performance statistics of the node to the collector, as
// the pushstats action of the v2 agent did: cpu, memory, swap, load, block and
// network i/o read from sysstat, and the file system usage.
func (t *Node) PushStats(ctx context.Context, o PushStatsOptions) (PushStatsResult, error) {
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return PushStatsResult{}, err
	}
	return t.pushStatsRun(ctx, o, func(ctx context.Context, stats nodestats.Stats, result *PushStatsResult) error {
		stored, err := pushStatsSend(ctx, oc3, stats)
		if err != nil {
			return err
		}
		result.Series += stored.Series
		result.Points += stored.Points
		for _, s := range stored.Skipped {
			if !slices.Contains(result.Skipped, s) {
				result.Skipped = append(result.Skipped, s)
			}
		}
		return nil
	})
}

// pushStatsSend sends a batch of statistics to the collector, and returns
// what it stored.
func pushStatsSend(ctx context.Context, oc3 *httphelper.T, stats nodestats.Stats) (oc3NodeStatsStored, error) {
	var stored oc3NodeStatsStored
	b, err := json.Marshal(oc3NodeStatsBody{Data: stats})
	if err != nil {
		return stored, fmt.Errorf("encode request body: %w", err)
	}
	method, path := http.MethodPost, oc3path.FeedNodeStats
	ctx, cancel := context.WithTimeout(ctx, pushStatsTimeout)
	defer cancel()
	req, err := oc3.NewRequestWithContext(ctx, method, path, bytes.NewReader(b))
	if err != nil {
		return stored, fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	resp, err := oc3.Do(req)
	if err != nil {
		return stored, fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := CollectorResponseStatusCheck(resp, method, path, []int{http.StatusOK}); err != nil {
		return stored, err
	}
	if err := json.NewDecoder(resp.Body).Decode(&stored); err != nil {
		return stored, fmt.Errorf("decode collector response: %w", err)
	}
	return stored, nil
}

// pushStatsTime is a time as the commands print one: RFC 3339, in the local
// time zone, to the second.
func pushStatsTime(t time.Time) string {
	return t.Local().Truncate(time.Second).Format(time.RFC3339)
}

// Render is the result as a person reads it: the range read, how many rows
// of each group, and what the collector stored.
func (t PushStatsResult) Render() string {
	var b strings.Builder
	line := func(name, value string) {
		if value != "" {
			fmt.Fprintf(&b, "%-9s %s\n", name, value)
		}
	}
	line("range", pushStatsTime(t.Begin)+" to "+pushStatsTime(t.End))
	line("batches", strconv.Itoa(t.Batches))
	groups := slices.Sorted(maps.Keys(t.Rows))
	rows := make([]string, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, fmt.Sprintf("%s %d", group, t.Rows[group]))
	}
	line("rows", strings.Join(rows, ", "))
	if t.DryRun {
		line("stored", "nothing, this is a dry run")
	} else {
		line("stored", fmt.Sprintf("%d series, %d points", t.Series, t.Points))
	}
	line("skipped", strings.Join(t.Skipped, ", "))
	line("disabled", strings.Join(t.Disabled, ", "))
	return b.String()
}
