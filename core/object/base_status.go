package object

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/file"
)

// OptsStatus is the options of the Start object method.
type OptsStatus struct {
	Global  OptsGlobal
	Lock    OptsLocking
	Refresh bool `flag:"refresh"`
	//Status string `flag:"status"`
}

func (t *Base) statusFile() string {
	return filepath.Join(t.varDir(), "status.json")
}

// Status returns the service status dataset
func (t *Base) Status(options OptsStatus) (InstanceStatus, error) {
	var (
		data InstanceStatus
		err  error
	)
	if options.Refresh || t.statusDumpOutdated() {
		return t.statusEval()
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval()
}

func (t *Base) statusEval() (InstanceStatus, error) {
	data := InstanceStatus{}
	err := errors.New("Not implemented") // Simulate err to avoid dumping over status.json
	if err != nil {
		return data, err
	}
	t.statusDump(data)
	return data, nil
}

func (t *Base) statusDumpOutdated() bool {
	return t.statusDumpModTime().Before(t.configModTime())
}

func (t *Base) configModTime() time.Time {
	p := t.statusFile()
	return file.ModTime(p)
}

func (t *Base) statusDumpModTime() time.Time {
	p := t.statusFile()
	return file.ModTime(p)
}

func (t *Base) statusDump(data InstanceStatus) error {
	p := t.statusFile()
	tmp := "." + p + ".swp"
	jsonFile, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(jsonFile)
	err = enc.Encode(data)
	if err != nil {
		t.log.Error().
			Str("file", tmp).
			Err(err).
			Msg("")
		return err
	}
	return os.Rename(tmp, p)
}

func (t *Base) statusLoad() (InstanceStatus, error) {
	data := InstanceStatus{}
	p := t.statusFile()
	jsonFile, err := os.Open(p)
	if err != nil {
		return data, err
	}
	defer jsonFile.Close()
	dec := json.NewDecoder(jsonFile)
	err = dec.Decode(&data)
	if err != nil {
		t.log.Error().
			Str("file", p).
			Err(err).
			Msg("")
		return data, err
	}
	return data, err
}
