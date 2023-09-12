package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/actioncontext"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/api"
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
	data.UpdatedAt = time.Now()
	data.Running = runningRIDList(t)
	data.Avail = status.NotApplicable
	data.Overall = status.NotApplicable
	data.Optional = status.NotApplicable
	err = t.statusDump(data)
	return
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
		t.log.Error().Str("file", tmp).Err(err).Send()
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		t.log.Error().Str("file", tmp).Err(err).Send()
		return err
	}
	t.log.Debug().Str("file", p).Msg("dumped")
	if err := t.postInstanceStatus(data); err != nil {
		// daemon can be down
		t.log.Debug().Err(err).Msg("post instance status")
	} else {
		t.log.Debug().Msg("posted instance status")
	}
	return nil
}

func (t *core) postInstanceStatus(data instance.Status) error {
	var (
		instanceStatus api.InstanceStatus
		b              []byte
	)
	buff := bytes.NewBuffer(b)
	if err := json.NewEncoder(buff).Encode(data); err != nil {
		return err
	}
	if err := json.NewDecoder(buff).Decode(&instanceStatus); err != nil {
		return err
	}
	if c, err := client.New(); err != nil {
		return err
	} else {
		body := api.InstanceStatusItem{
			Meta: api.InstanceMeta{
				Object: t.path.String(),
				Node:   hostname.Hostname(),
			},
			Data: instanceStatus,
		}
		resp, err := c.PostInstanceStatusWithResponse(context.Background(), body)
		if err != nil {
			return err
		}
		switch resp.StatusCode() {
		case 200:
		case 400:
			return fmt.Errorf("%s", resp.JSON400)
		case 401:
			return fmt.Errorf("%s", resp.JSON401)
		case 403:
			return fmt.Errorf("%s", resp.JSON403)
		case 500:
			return fmt.Errorf("%s", resp.JSON500)
		default:
			return fmt.Errorf("unexpected response: %s", string(resp.Body))
		}
		return nil
	}
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
			Send()
		return data, err
	}
	return data, err
}
