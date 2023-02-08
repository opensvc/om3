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
	"time"

	"github.com/ssrathi/go-attr"
	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
)

func (t *core) statusFile() string {
	return filepath.Join(t.varDir(), "status.json")
}

func (t *core) FreshStatus(ctx context.Context) (instance.Status, error) {
	return t.statusEval(ctx)
}

func (t *core) Status(ctx context.Context) (instance.Status, error) {
	var (
		data instance.Status
		err  error
	)
	if t.statusDumpOutdated() {
		return t.statusEval(ctx)
	}
	if data, err = t.statusLoad(); err == nil {
		return data, nil
	}
	// corrupted status.json => eval
	return t.statusEval(ctx)
}

func (t *core) statusEval(ctx context.Context) (instance.Status, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.Status)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return instance.Status{}, err
	}
	defer unlock()
	return t.lockedStatusEval()
}

func (t *core) lockedStatusEval() (data instance.Status, err error) {
	data.App = t.App()
	data.Env = t.Env()
	data.Kind = t.path.Kind
	data.Updated = time.Now()
	data.Parents = t.Parents()
	data.Children = t.Children()
	data.DRP = t.config.IsInDRPNodes(hostname.Hostname())
	data.Running = runningRIDList(t)
	data.Avail = status.NotApplicable
	data.Overall = status.NotApplicable
	data.Optional = status.NotApplicable
	data.Csum = csumStatusData(data)
	t.statusDump(data)
	return
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

// csumStatusData returns the string representation of the checksum of the
// status.json content, adding recursively all data keys except
// time and checksum fields.
func csumStatusData(data instance.Status) string {
	w := md5.New()
	if err := csumStatusDataRecurse(w, data); err != nil {
		fmt.Println(data, err) // TODO: remove me
	}
	return fmt.Sprintf("%x", w.Sum(nil))
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

// statusFilePair returns a pair of file path suitable for a tmp-to-final move
// after change.
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
