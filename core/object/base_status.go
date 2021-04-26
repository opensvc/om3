package object

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/timestamp"
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
		return t.statusEval(options)
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval(options)
}

func (t *Base) statusEval(options OptsStatus) (data InstanceStatus, err error) {
	lockErr := t.lockedAction("status", options.Lock.Timeout, "", func() error {
		data, err = t.lockedStatusEval()
		return err
	})
	if lockErr != nil {
		err = lockErr
	}
	return
}

func (t *Base) lockedStatusEval() (data InstanceStatus, err error) {
	data.App = t.App()
	data.Env = t.Env()
	data.Topology = t.Topology()
	data.Placement = t.Placement()
	data.Priority = t.Priority()
	data.Kind = t.Path.Kind
	data.Updated = timestamp.Now()
	if err = t.resourceStatusEval(&data); err != nil {
		return
	}
	if data.Topology == topology.Flex {
		data.FlexTarget = t.FlexTarget()
		data.FlexMin = t.FlexMin()
		data.FlexMax = t.FlexMax()
	}
	t.statusDump(data)
	return
}

func (t *Base) resourceStatusEval(data *InstanceStatus) error {
	data.Resources = make(map[string]resource.ExposedStatus)
	return t.ResourceSets().Do(t, func(r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("stat resource")
		xd := resource.GetExposedStatus(r)
		data.Resources[r.RID()] = xd
		if xd.Optional {
			data.Overall.Add(xd.Status)
		} else {
			data.Avail.Add(xd.Status)
		}
		return nil
	})
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

//
// statusFilePair returns a pair of file path suitable for a tmp-to-final move
// after change.
//
func (t Base) statusFilePair() (final string, tmp string) {
	final = t.statusFile()
	tmp = filepath.Join(filepath.Dir(final), "."+filepath.Base(final)+".swp")
	return
}

func (t *Base) statusDump(data InstanceStatus) error {
	p, tmp := t.statusFilePair()
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
