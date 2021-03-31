package check

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/exe"
)

type (
	// Runner exposes the method to run the check drivers,
	// aggregate results and format the output.
	Runner struct{}
)

// Do runs the check drivers, aggregates results and format
// the output.
func (r Runner) Do() *ResultSet {
	rs := NewResultSet()
	q := make(chan *ResultSet)
	paths := r.list()
	for _, path := range paths {
		go doCheck(q, path)
	}
	for _, c := range checkers {
		go doRegisteredCheck(q, c)
	}
	for range paths {
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
		Int("drivers", len(paths)).
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

func doCheck(q chan *ResultSet, path string) {
	rs := NewResultSet()
	cmd := exec.Command(path)
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		log.Error().Str("checker", path).Err(err).Msg("execution")
		q <- rs
		return
	}
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

func (r Runner) list() []string {
	l := make([]string, 0)
	root := filepath.Join(config.NodeViper.GetString("paths.drivers"), "check")
	log.Debug().
		Str("c", "checks").
		Str("head", root).
		Msg("search check drivers")
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsDir() {
			return nil
		}
		if exe.IsExecOwner(info.Mode().Perm()) {
			l = append(l, path)
			return nil
		}
		return nil
	})
	return l
}
