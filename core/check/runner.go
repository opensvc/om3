package check

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/v3/util/funcopt"
)

var ExecCommand = exec.CommandContext

type (
	// Runner collects results and format the output.
	Runner struct {
		customCheckPaths []string
		objects          []interface{}
		q                chan *ResultSet
	}
)

func NewRunner(opts ...funcopt.O) *Runner {
	r := &Runner{
		q: make(chan *ResultSet),
	}
	_ = funcopt.Apply(r, opts...)
	return r
}

// RunnerWithCustomCheckPaths adds paths where additional check
// driver are installed.
func RunnerWithCustomCheckPaths(paths ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Runner)
		t.customCheckPaths = append(t.customCheckPaths, paths...)
		return nil
	})
}

// RunnerWithObjects sets the list of objects the checkers can
// use to correlate a check instance to an object.
func RunnerWithObjects(objs ...interface{}) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*Runner)
		t.objects = append(t.objects, objs...)
		return nil
	})
}

// Do runs the check drivers, aggregates results and format
// the output.
func (r Runner) Do(ctx context.Context, opts ...funcopt.O) *ResultSet {
	rs := NewResultSet()
	defer forgetIndexes(r.objects)
	for _, path := range r.customCheckPaths {
		go r.doCustomCheck(ctx, path)
	}
	for _, c := range checkers {
		go r.doRegisteredCheck(ctx, c)
	}
	for range r.customCheckPaths {
		d := <-r.q
		rs.Add(d)
	}
	for range checkers {
		d := <-r.q
		rs.Add(d)
	}
	log.Debug().
		Str("c", "checks").
		Int("instances", len(rs.Data)).
		Int("drivers", len(r.customCheckPaths)).
		Msgf("checks done: %d results, %d custom checkers", len(rs.Data), len(r.customCheckPaths))
	return rs
}

func (r *Runner) doRegisteredCheck(ctx context.Context, c Checker) {
	begin := time.Now()
	rs, err := c.Check(ctx, r.objects)
	if rs == nil {
		// A checker failing before it has results, which the aggregation
		// can not add.
		rs = NewResultSet()
	}
	if err != nil {
		log.Error().Err(err).Msgf("checker %T: execution", c)
		r.q <- rs
		return
	}
	log.Debug().
		Str("c", "checks").
		Int("instances", len(rs.Data)).
		Msgf("checker %T: %d results in %s", c, len(rs.Data), time.Since(begin).Round(time.Millisecond))
	r.q <- rs
}

func (r *Runner) doCustomCheck(ctx context.Context, path string) {
	rs := NewResultSet()
	cmd := ExecCommand(ctx, path)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		log.Error().Str("checker", path).Err(err).Msgf("checker %s: execution", path)
		r.q <- rs
		return
	}
	log.Debug().Str("checker", path).Msgf("checker %s: %s", path, b)
	if err := json.Unmarshal(b, rs); err != nil {
		log.Error().Str("checker", path).Err(err).Msgf("checker %s: unmarshal json", path)
	}
	log.Debug().
		Str("c", "checks").
		Str("driver", path).
		Int("instances", len(rs.Data)).
		Msgf("checker %s: %d results", path, len(rs.Data))
	r.q <- rs
}
