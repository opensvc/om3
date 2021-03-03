package check

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/output"
	"opensvc.com/opensvc/util/exe"
)

type (
	// Runner exposes the method to run the check drivers,
	// aggregate results and format the output.
	Runner struct {
		Color  string
		Format string
	}
)

// Do runs the check drivers, aggregates results and format
// the output.
func (r Runner) Do() {
	data := make([]Result, 0)
	q := make(chan []Result)
	paths := r.list()
	for _, path := range paths {
		go doCheck(q, path)
	}
	for range paths {
		d := <-q
		data = append(data, d...)
	}
	output.Renderer{
		Color:  r.Color,
		Format: r.Format,
		Data:   data,
	}.Print()
}

func doCheck(q chan []Result, path string) {
	var results []Result
	if b, err := exec.Command(path).Output(); err == nil {
		json.Unmarshal(b, &results)
	}
	q <- results
}

func (r Runner) list() []string {
	l := make([]string, 0)
	root := filepath.Join(config.Viper.GetString("paths.drivers"), "check")
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
