package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/opensvc/om3/v3/core/oc3path"
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
		Begin    time.Time `json:"begin"`
		End      time.Time `json:"end"`
		Groups   []string  `json:"groups"`
		Series   int       `json:"series"`
		Points   int       `json:"points"`
		Skipped  []string  `json:"skipped,omitempty"`
		Disabled []string  `json:"disabled,omitempty"`
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

// collectStats reads the statistics a push would send, but for the groups the
// stats.disable keyword lists.
func (t *Node) collectStats(ctx context.Context, o PushStatsOptions) (nodestats.Stats, PushStatsResult, error) {
	last := t.newScheduleEntry("pushstats", "stats", "", "stats_push").LastRunAt
	begin, end := pushStatsRange(o, last, time.Now())
	disabled := t.MergedConfig().GetStrings(key.New("stats", "disable"))
	result := PushStatsResult{Begin: begin, End: end, Disabled: disabled}
	stats, err := nodestats.Collect(ctx, nodestats.Options{Begin: begin, End: end, Dir: o.StatsDir, Disable: disabled})
	if err != nil {
		return nil, result, err
	}
	result.Groups = stats.Summary()
	return stats, result, nil
}

// PushStatsDryRun reads the statistics a push would send, and sends nothing.
func (t *Node) PushStatsDryRun(ctx context.Context, o PushStatsOptions) (PushStatsResult, error) {
	_, result, err := t.collectStats(ctx, o)
	return result, err
}

// PushStats sends the performance statistics of the node to the collector, as
// the pushstats action of the v2 agent did: cpu, memory, swap, load, block and
// network i/o read from sysstat, and the file system usage.
func (t *Node) PushStats(ctx context.Context, o PushStatsOptions) (PushStatsResult, error) {
	stats, result, err := t.collectStats(ctx, o)
	if err != nil {
		return result, err
	}
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return result, err
	}
	b, err := json.Marshal(oc3NodeStatsBody{Data: stats})
	if err != nil {
		return result, fmt.Errorf("encode request body: %w", err)
	}
	method, path := http.MethodPost, oc3path.FeedNodeStats
	ctx, cancel := context.WithTimeout(ctx, pushStatsTimeout)
	defer cancel()
	req, err := oc3.NewRequestWithContext(ctx, method, path, bytes.NewReader(b))
	if err != nil {
		return result, fmt.Errorf("create collector request %s %s: %w", method, path, err)
	}
	resp, err := oc3.Do(req)
	if err != nil {
		return result, fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := CollectorResponseStatusCheck(resp, method, path, []int{http.StatusOK}); err != nil {
		return result, err
	}
	var stored oc3NodeStatsStored
	if err := json.NewDecoder(resp.Body).Decode(&stored); err != nil {
		return result, fmt.Errorf("decode collector response: %w", err)
	}
	result.Series, result.Points, result.Skipped = stored.Series, stored.Points, stored.Skipped
	return result, nil
}
