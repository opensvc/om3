package check

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/rs/zerolog/log"
)

var ExecCommand = exec.Command

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
func (r Runner) Do(opts ...funcopt.O) *ResultSet {
	rs := NewResultSet()
	for _, path := range r.customCheckPaths {
		go r.doCustomCheck(path)
	}
	for _, c := range checkers {
		go r.doRegisteredCheck(c)
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
		Msg("checks done")
	return rs
}

func (r *Runner) doRegisteredCheck(c Checker) {
	rs, err := c.Check(r.objects)
	if err != nil {
		log.Error().Err(err).Msg("execution")
		r.q <- rs
		return
	}
	log.Debug().
		Str("c", "checks").
		Int("instances", len(rs.Data)).
		Send()
	r.q <- rs
}

func (r *Runner) doCustomCheck(path string) {
	rs := NewResultSet()
	cmd := ExecCommand(path)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		log.Error().Str("checker", path).Err(err).Msg("execution")
		r.q <- rs
		return
	}
	log.Error().Str("checker", path).Err(err).Msg(string(b))
	if err := json.Unmarshal(b, rs); err != nil {
		log.Error().Str("checker", path).Err(err).Msg("unmarshal json")
	}
	log.Debug().
		Str("c", "checks").
		Str("driver", path).
		Int("instances", len(rs.Data)).
		Send()
	r.q <- rs
}
