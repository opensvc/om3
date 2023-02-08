package check

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"
	"github.com/opensvc/om3/util/funcopt"
)

var ExecCommand = exec.Command

type (
	// aggregate results and format the output.
	runner struct {
		customCheckPaths []string
		objects          []interface{}
		q                chan *ResultSet
	}
)

func NewRunner(opts ...funcopt.O) *runner {
	r := &runner{
		q: make(chan *ResultSet),
	}
	_ = funcopt.Apply(r, opts...)
	return r
}

// RunnerWithCustomCheckPaths adds paths where additionnal check
// driver are installed.
func RunnerWithCustomCheckPaths(paths ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*runner)
		t.customCheckPaths = append(t.customCheckPaths, paths...)
		return nil
	})
}

// RunnerWithObjects sets the list of objects the checkers can
// use to correlate a check instance to an object.
func RunnerWithObjects(objs ...interface{}) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*runner)
		t.objects = append(t.objects, objs...)
		return nil
	})
}

// Do runs the check drivers, aggregates results and format
// the output.
func (r runner) Do(opts ...funcopt.O) *ResultSet {
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

func (r *runner) doRegisteredCheck(c Checker) {
	rs, err := c.Check(r.objects)
	if err != nil {
		log.Error().Err(err).Msg("execution")
		r.q <- rs
		return
	}
	log.Debug().
		Str("c", "checks").
		Int("instances", len(rs.Data)).
		Msg("")
	r.q <- rs
}

func (r *runner) doCustomCheck(path string) {
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
		Msg("")
	r.q <- rs
}
