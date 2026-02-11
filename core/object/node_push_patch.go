package object

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/rawconfig"

	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/patches"
)

func (t Node) nodePatchCacheFile() string {
	return filepath.Join(rawconfig.NodeVarDir(), "patch.json")
}

func (t Node) PushPatch() ([]patches.Patch, error) {
	l, err := patches.List()
	if err != nil {
		return l, err
	}
	if len(l) == 0 {
		return l, nil
	}
	if err := t.dumpPatch(l); err != nil {
		return l, err
	}
	if err := t.pushPatch(l); err != nil {
		return l, err
	}
	return l, nil
}

func (t Node) dumpPatch(data []patches.Patch) error {
	file, err := os.OpenFile(t.nodePatchCacheFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return json.NewEncoder(file).Encode(data)
}

func (t Node) LoadPatch() ([]patches.Patch, error) {
	var data []patches.Patch
	file, err := os.Open(t.nodePatchCacheFile())
	if err != nil {
		return data, err
	}
	defer func() { _ = file.Close() }()
	err = json.NewDecoder(file).Decode(&data)
	return data, err
}

func (t Node) pushPatch(data []patches.Patch) error {
	nodename := hostname.Hostname()
	patchAsList := func(t patches.Patch) []string {
		installedAt := ""
		if !t.InstalledAt.IsZero() {
			installedAt = t.InstalledAt.Format("2006-01-02 15:04:05")
		}
		return []string{
			nodename,
			t.Number,
			t.Revision,
			installedAt,
		}
	}
	patchsAsList := func(t []patches.Patch) [][]string {
		l := make([][]string, len(t))
		for i, p := range t {
			l[i] = patchAsList(p)
		}
		return l
	}
	vars := []string{
		"patch_nodename",
		"patch_num",
		"patch_rev",
		"patch_install_date",
	}
	client, err := t.CollectorFeedClient()
	if err != nil {
		return err
	}
	if response, err := client.Call("insert_patch", vars, patchsAsList(data)); err != nil {
		return err
	} else if response.Error != nil {
		return fmt.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}
	return nil
}
