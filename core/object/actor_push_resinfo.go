package object

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceselector"
)

type (
	postResInfo struct {
		Info     []resource.Info `json:"info"`
		Path     string          `json:"path"`
		Topology *string         `json:"topology,omitempty"`
	}
)

// PushResInfo pushes resources information of the local instance of the object
func (t *actor) PushResInfo(ctx context.Context) (resource.Infos, error) {
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
	return t.lockedPushResInfo(ctx)
}

func (t *actor) lockedPushResInfo(ctx context.Context) (resource.Infos, error) {
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
	}
	return infos, t.collectorPushResInfo(infos)
}

func (t *actor) resInfoCacheFilename() string {
	return filepath.Join(t.varDir(), "resinfo.json")
}

func (t *actor) LoadResInfo() (resource.Infos, error) {
	var data resource.Infos
	filename := t.resInfoCacheFilename()
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
	filename := t.resInfoCacheFilename()
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
		if !r.IsConfigured() {
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

func (t *actor) collectorPushResInfo(infos resource.Infos) error {
	var (
		req  *http.Request
		resp *http.Response

		ioReader io.Reader

		method = http.MethodPost
		path   = oc3path.FeedInstanceResinfo
	)
	node, err := t.Node()
	if err != nil {
		return err
	}
	oc3, err := node.CollectorClient()
	if err != nil {
		return err
	}

	topology := t.Topology().String()
	data := postResInfo{
		Info:     infos.Resources,
		Path:     infos.ObjectPath.String(),
		Topology: &topology,
	}

	if b, err := json.Marshal(data); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	req, err = oc3.NewRequestWithContext(context.Background(), method, path, ioReader)
	if err != nil {
		return fmt.Errorf("%s %s create request: %w", method, path, err)
	}

	resp, err = oc3.Do(req)
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d",
			method, path, http.StatusAccepted, resp.StatusCode)
	}
	return nil
}
