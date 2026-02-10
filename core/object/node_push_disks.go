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

	"github.com/opensvc/om3/v3/core/oc3path"
	"github.com/opensvc/om3/v3/core/rawconfig"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/disks"
)

type (
	oc3Disk struct {
		ID         string `json:"id"`
		ObjectPath string `json:"object_path"`
		Size       uint64 `json:"size"`
		Used       uint64 `json:"used"`
		Vendor     string `json:"vendor"`
		Model      string `json:"model"`
		Group      string `json:"group"`
		Nodename   string `json:"nodename"`
		Region     string `json:"region"`
	}

	oc3DiskBody struct {
		Data []oc3Disk `json:"data"`
	}
)

func (t Node) nodeDisksCacheFile() string {
	return filepath.Join(rawconfig.NodeVarDir(), "disks.json")
}

func allObjectsDeviceClaims() (disks.ObjectsDeviceClaims, error) {
	claims := disks.NewObjectsDeviceClaims()
	paths, err := naming.InstalledPaths()
	if err != nil {
		return claims, err
	}
	objs, err := NewList(paths.Filter("*/svc/*").Merge(paths.Filter("*/vol/*")), WithVolatile(true))
	if err != nil {
		return claims, err
	}
	claims.AddObjects(objs...)
	return claims, err
}

func (t Node) PushDisks() (disks.Disks, error) {
	claims, err := allObjectsDeviceClaims()
	if err != nil {
		return nil, err
	}
	t.Log().Attr("claims", claims).Tracef("PushDisks %s", claims)
	l, err := disks.GetDisks(claims)
	if err != nil {
		return l, err
	}
	if err := t.dumpDisks(l); err != nil {
		return l, err
	}
	if err := t.pushDisks(l); err != nil {
		return l, err
	}
	return l, nil
}

func (t Node) dumpDisks(data disks.Disks) error {
	file, err := os.OpenFile(t.nodeDisksCacheFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return json.NewEncoder(file).Encode(data)
}

func (t Node) LoadDisks() (disks.Disks, error) {
	var data disks.Disks
	file, err := os.Open(t.nodeDisksCacheFile())
	if err != nil {
		return data, err
	}
	defer func() { _ = file.Close() }()
	err = json.NewDecoder(file).Decode(&data)
	return data, err
}

func toOc3Disk(l disks.Disks) []oc3Disk {
	result := make([]oc3Disk, 0)
	for _, dsk := range l {
		for _, region := range dsk.Regions {
			result = append(result, oc3Disk{
				ID:         dsk.ID,
				ObjectPath: region.Object,
				Size:       region.Size / 1024 / 1024,
				Used:       region.Size / 1024 / 1024,
				Vendor:     dsk.Vendor,
				Model:      dsk.Model,
				Group:      region.Group,
				Region:     "0",
			})
		}
	}
	return result
}

func (t Node) pushDisks(data disks.Disks) error {
	var (
		req  *http.Request
		resp *http.Response

		ioReader io.Reader

		method = http.MethodPost
		path   = oc3path.FeedNodeDisk
	)
	oc3, err := t.CollectorFeeder()
	if err != nil {
		return err
	}
	if b, err := json.MarshalIndent(oc3DiskBody{Data: toOc3Disk(data)}, "  ", "  "); err != nil {
		return fmt.Errorf("encode request body: %w", err)
	} else {
		ioReader = bytes.NewBuffer(b)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultPostCollectorTimeout)
	defer cancel()

	req, err = oc3.NewRequestWithContext(ctx, method, path, ioReader)
	if err != nil {
		return fmt.Errorf("create collector request %s %s: %w", method, path, err)
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
