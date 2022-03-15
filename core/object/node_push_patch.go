package object

import (
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/patches"
)

type (
	// OptsNodePushPatch is the options of the PushAsset function.
	OptsNodePushPatch struct {
		Global OptsGlobal
	}
)

func (t Node) PushPatch() ([]patches.Patch, error) {
	l, err := patches.List()
	if err != nil {
		return l, err
	}
	if err := t.pushPatch(l); err != nil {
		return l, err
	}
	return l, nil
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
	client, err := t.collectorClient()
	if err != nil {
		return err
	}
	if response, err := client.Call("insert_patch", vars, patchsAsList(data)); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}
	return nil
}
