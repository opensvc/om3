package object

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
	"github.com/opensvc/om3/v3/util/hostname"
)

// RefreshResInfo refreshes the resource info cache of the local instance of the
// object.
//
// It does not feed the collector: once the cache is written, the local daemon is
// signalled, and the collector speaker reports the key-values it fetches from
// this node on its own throttled schedule.
func (t *actor) RefreshResInfo(ctx context.Context) (resource.Infos, error) {
	ctx = actioncontext.WithProps(ctx, actioncontext.PushResInfo)
	if err := t.validateAction(); err != nil {
		return resource.Infos{}, err
	}
	t.setenv("push resinfo", false)
	unlock, err := t.lockAction(ctx)
	if err != nil {
		return resource.Infos{}, err
	}
	defer unlock()
	return t.lockedRefreshResInfo(ctx)
}

func (t *actor) lockedRefreshResInfo(ctx context.Context) (resource.Infos, error) {
	infos := resource.NewInfos(t.Path())
	if more, err := t.masterResInfo(ctx); err != nil {
		return infos, err
	} else {
		infos.Resources = append(infos.Resources, more...)
	}
	if more, err := t.slaveResInfo(ctx); err != nil {
		return infos, err
	} else {
		infos.Resources = append(infos.Resources, more...)
	}
	if err := t.saveResInfo(infos); err != nil {
		t.log.Warnf("%s", err)
		return infos, nil
	}
	if err := t.postResInfoSignal(); err != nil {
		// daemon can be down
		t.log.Tracef("post resource info signal error: %s", err)
	} else {
		t.log.Tracef("posted resource info signal")
	}
	return infos, nil
}

// ResInfoCacheFile returns the path of the file caching the resource info
// key-values of the local instance.
func (t *actor) ResInfoCacheFile() string {
	return filepath.Join(t.varDir(), "resinfo.json")
}

func (t *actor) LoadResInfo() (resource.Infos, error) {
	var data resource.Infos
	filename := t.ResInfoCacheFile()
	file, err := os.Open(filename)
	if err != nil {
		return data, err
	}
	defer file.Close()
	dec := json.NewDecoder(file)
	err = dec.Decode(&data)
	return data, err
}

func (t *actor) saveResInfo(data resource.Infos) error {
	filename := t.ResInfoCacheFile()
	tempFile, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".*")
	if err != nil {
		return err
	}
	tempFilename := tempFile.Name()
	enc := json.NewEncoder(tempFile)
	if err := enc.Encode(data); err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()
	return os.Rename(tempFilename, filename)
}

func (t *actor) masterResInfo(ctx context.Context) ([]resource.Info, error) {
	l := make([]resource.Info, 0)
	resourceLister := resourceselector.FromContext(ctx, t)
	barrier := actioncontext.To(ctx)
	err := t.ResourceSets().Do(ctx, resourceLister, barrier, "resinfo", func(ctx context.Context, r resource.Driver) error {
		if r.GetConfigurationError() != nil {
			return nil
		}
		info, err := resource.GetInfo(ctx, r)
		if err != nil {
			return err
		}
		l = append(l, info)
		return nil
	})
	return l, err
}

func (t *actor) slaveResInfo(ctx context.Context) ([]resource.Info, error) {
	return []resource.Info{}, nil
}

// postResInfoSignal tells the local daemon the resource info cache has been
// refreshed, so it publishes a msgbus.InstanceResourceInfoUpdated the collector
// speaker subscribes to.
func (t *actor) postResInfoSignal() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	p := t.Path()
	resp, err := c.PostInstanceResourceInfoWithResponse(context.Background(), hostname.Hostname(), p.Namespace, p.Kind, p.Name)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		return nil
	default:
		return fmt.Errorf("unexpected response: %s", string(resp.Body))
	}
}
