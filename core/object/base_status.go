package object

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/ssrathi/go-attr"
	"opensvc.com/opensvc/core/actioncontext"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/statusbus"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/timestamp"
)

// OptsStatus is the options of the Start object method.
type OptsStatus struct {
	OptsLock
	Refresh bool `flag:"refresh"`
	//Status string `flag:"status"`
}

func (t *core) statusFile() string {
	return filepath.Join(t.varDir(), "status.json")
}

// Status returns the service status dataset
func (t *core) Status(options OptsStatus) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	ctx := context.Background()
	ctx = actioncontext.WithOptions(ctx, options)
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	ctx, stop := statusbus.WithContext(ctx, t.path)
	defer stop()

	if options.Refresh || t.statusDumpOutdated() {
		return t.statusEval(ctx, options)
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval(ctx, options)
}

func (t *core) postActionStatusEval(ctx context.Context) {
	if _, err := t.statusEval(ctx, OptsStatus{}); err != nil {
		t.log.Debug().Err(err).Msg("a status refresh is already in progress")
	}
}

func (t *core) statusEval(ctx context.Context, options OptsStatus) (instance.Status, error) {
	props := actioncontext.Status
	unlock, err := t.lockAction(props, options.OptsLock)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedStatusEval(ctx)
}

func (t *core) lockedStatusEval(ctx context.Context) (data instance.Status, err error) {
	data.App = t.App()
	data.Env = t.Env()
	data.Orchestrate = t.Orchestrate()
	data.Topology = t.Topology()
	data.Placement = t.Placement()
	data.Priority = t.Priority()
	data.Kind = t.path.Kind
	data.Updated = timestamp.Now()
	data.Parents = t.Parents()
	data.Children = t.Children()
	data.DRP = t.config.IsInDRPNodes(hostname.Hostname())
	data.Subsets = t.subsetsStatus()
	data.Frozen = t.Frozen()
	data.Running = t.RunningRIDList()
	if err = t.resourceStatusEval(ctx, &data); err != nil {
		return
	}
	if len(data.Resources) == 0 {
		data.Avail = status.NotApplicable
		data.Overall = status.NotApplicable
		data.Optional = status.NotApplicable
	}
	if data.Topology == topology.Flex {
		data.FlexTarget = t.FlexTarget()
		data.FlexMin = t.FlexMin()
		data.FlexMax = t.FlexMax()
	}
	data.Csum = csumStatusData(data)
	t.statusDump(data)
	return
}

func (t *core) RunningRIDList() []string {
	l := make([]string, 0)
	for _, r := range t.Resources() {
		if i, ok := r.(resource.IsRunninger); !ok {
			continue
		} else if !i.IsRunning() {
			continue
		}
		l = append(l, r.RID())
	}
	return l
}

func csumStatusDataRecurse(w io.Writer, d interface{}) error {
	names, err := attr.Names(d)
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		kind, err := attr.GetKind(d, name)
		if err != nil {
			return err
		}
		switch name {
		case "StatusUpdated", "GlobalExpectUpdated", "Updated", "Mtime", "Csum":
			continue
		}
		val, err := attr.GetValue(d, name)
		if err != nil {
			return err
		}
		switch kind {
		case "struct":
			if err := csumStatusDataRecurse(w, val); err != nil {
				return err
			}
		case "slice":
			rv := reflect.ValueOf(val)
			for i := 0; i < rv.Len(); i++ {
				v := rv.Index(i)
				if err := csumStatusDataRecurse(w, v); err != nil {
					return err
				}
			}
		case "map":
			iter := reflect.ValueOf(val).MapRange()
			for iter.Next() {
				// k := iter.Key()
				v := iter.Value()
				if err := csumStatusDataRecurse(w, v); err != nil {
					return err
				}
			}
		default:
			fmt.Fprint(w, val)
		}
	}
	return nil
}

//
// csumStatusData returns the string representation of the checksum of the
// status.json content, adding recursively all data keys except
// timestamp and checksum fields.
//
func csumStatusData(data instance.Status) string {
	w := md5.New()
	if err := csumStatusDataRecurse(w, data); err != nil {
		fmt.Println(data, err) // TODO: remove me
	}
	return fmt.Sprintf("%x", w.Sum(nil))
}

func (t *core) subsetsStatus() map[string]instance.SubsetStatus {
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

func (t *core) resourceStatusEval(ctx context.Context, data *instance.Status) error {
	data.Resources = make(map[string]resource.ExposedStatus)
	var mu sync.Mutex
	return t.ResourceSets().Do(ctx, t, "", func(ctx context.Context, r resource.Driver) error {
		xd := resource.GetExposedStatus(ctx, r)
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

func (t *core) statusDumpOutdated() bool {
	return t.statusDumpModTime().Before(t.configModTime())
}

func (t *core) configModTime() time.Time {
	p := t.ConfigFile()
	return file.ModTime(p)
}

func (t *core) statusDumpModTime() time.Time {
	p := t.statusFile()
	return file.ModTime(p)
}

//
// statusFilePair returns a pair of file path suitable for a tmp-to-final move
// after change.
//
func (t core) statusFilePair() (final string, tmp string) {
	final = t.statusFile()
	tmp = filepath.Join(filepath.Dir(final), "."+filepath.Base(final)+".swp")
	return
}

func (t *core) statusDump(data instance.Status) error {
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

func (t *core) postObjectStatus(data instance.Status) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	req := c.NewPostObjectStatus()
	req.Path = t.path.String()
	req.Data = data
	_, err = req.Do()
	return err
}

func (t *core) statusLoad() (instance.Status, error) {
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
