package check

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
)

var execCommand = exec.Command

type (
	// aggregate results and format the output.
	runner struct {
		customCheckPaths []string
	}
)

func NewRunner(customCheckPaths []string) *runner {
	return &runner{
		customCheckPaths: customCheckPaths,
	}
}

// Do runs the check drivers, aggregates results and format
// the output.
func (r runner) Do() *ResultSet {
	rs := NewResultSet()
	q := make(chan *ResultSet)
	for _, path := range r.customCheckPaths {
		go doCustomCheck(q, path)
	}
	for _, c := range checkers {
		go doRegisteredCheck(q, c)
	}
	for range r.customCheckPaths {
		d := <-q
		rs.Add(d)
	}
	for range checkers {
		d := <-q
		rs.Add(d)
	}
	log.Debug().
		Str("c", "checks").
		Int("instances", len(rs.Data)).
		Int("drivers", len(r.customCheckPaths)).
		Msg("checks done")
	return rs
}

func doRegisteredCheck(q chan *ResultSet, c Checker) {
	rs, err := c.Check()
	if err != nil {
		log.Error().Err(err).Msg("execution")
		q <- rs
		return
	}
	log.Debug().
		Str("c", "checks").
		Int("instances", len(rs.Data)).
		Msg("")
	q <- rs
}

func doCustomCheck(q chan *ResultSet, path string) {
	rs := NewResultSet()
	cmd := execCommand(path)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		log.Error().Str("checker", path).Err(err).Msg("execution")
		q <- rs
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
	q <- rs
}
