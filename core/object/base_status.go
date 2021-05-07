package object

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/instance"
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
func (t *Base) Status(options OptsStatus) (instance.Status, error) {
	var (
		data instance.Status
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

func (t *Base) postActionStatusEval() {
	if _, err := t.statusEval(OptsStatus{}); err != nil {
		t.log.Debug().Err(err).Msg("a status refresh is already in progress")
	}
}

func (t *Base) statusEval(options OptsStatus) (data instance.Status, err error) {
	lockErr := t.lockedAction("status", options.Lock, "", func() error {
		data, err = t.lockedStatusEval()
		return err
	})
	if lockErr != nil {
		err = lockErr
	}
	return
}

func (t *Base) lockedStatusEval() (data instance.Status, err error) {
	data.App = t.App()
	data.Env = t.Env()
	data.Orchestrate = t.Orchestrate()
	data.Topology = t.Topology()
	data.Placement = t.Placement()
	data.Priority = t.Priority()
	data.Kind = t.Path.Kind
	data.Updated = timestamp.Now()
	data.Parents = t.Parents()
	data.Children = t.Children()
	data.DRP = t.config.IsInDRPNodes(config.Node.Hostname)
	data.Subsets = t.subsetsStatus()
	data.Frozen = t.Frozen()
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

func (t *Base) subsetsStatus() map[string]instance.SubsetStatus {
	data := make(map[string]instance.SubsetStatus)
	for _, rs := range t.ResourceSets() {
		if !rs.Parallel {
			continue
		}
		data[rs.Fullname()] = instance.SubsetStatus{
			Parallel: rs.Parallel,
		}
	}
	return data
}

func (t *Base) resourceStatusEval(data *instance.Status) error {
	data.Resources = make(map[string]resource.ExposedStatus)
	var mu sync.Mutex
	return t.ResourceSets().Do(t, "", func(r resource.Driver) error {
		t.log.Debug().Str("rid", r.RID()).Msg("stat resource")
		xd := resource.GetExposedStatus(r)
		mu.Lock()
		data.Resources[r.RID()] = xd
		data.Overall.Add(xd.Status)
		if !xd.Optional {
			data.Avail.Add(xd.Status)
		}
		data.Provisioned.Add(xd.Provisioned.State)
		mu.Unlock()
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

func (t *Base) statusDump(data instance.Status) error {
	p, tmp := t.statusFilePair()
	jsonFile, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	enc := json.NewEncoder(jsonFile)
	err = enc.Encode(data)
	if err != nil {
		t.log.Error().Str("file", tmp).Err(err).Msg("")
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		t.log.Error().Str("file", tmp).Err(err).Msg("")
		return err
	}
	t.log.Debug().Str("file", p).Msg("dumped")
	_ = t.postObjectStatus(data)
	return nil
}

func (t *Base) postObjectStatus(data instance.Status) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	req := c.NewPostObjectStatus()
	req.Path = t.Path.String()
	req.Data = data
	_, err = req.Do()
	return err
}

func (t *Base) statusLoad() (instance.Status, error) {
	data := instance.Status{}
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
