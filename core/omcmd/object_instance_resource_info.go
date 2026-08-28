package omcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdObjectInstanceResourceInfo struct {
		OptsGlobal
		commoncmd.OptsLock
		NodeSelector string

		// Refresh recomputes the resource info cache before reporting it.
		// Without it, the cached key-values are reported as-is.
		Refresh bool
	}
)

func resourceInfosToAPI(infos resource.Infos, path, nodename string) api.ResourceInfoList {
	data := api.ResourceInfoList{
		Kind: "ResourceInfoList",
	}
	for _, r := range infos.Resources {
		for _, e := range r.Keys {
			item := api.ResourceInfoItem{
				Node:   nodename,
				Object: path,
				Rid:    r.RID,
				Key:    e.Key,
				Value:  e.Value,
			}
			data.Items = append(data.Items, item)
		}
	}
	return data
}

func (t *CmdObjectInstanceResourceInfo) extractLocal(selector string) (api.ResourceInfoList, error) {
	data := api.ResourceInfoList{
		Kind: "ResourceInfoList",
	}
	sel := objectselector.New(
		selector,
		objectselector.WithLocal(true),
	)
	type loadResInfoer interface {
		LoadResInfo() (resource.Infos, error)
	}
	type refreshResInfoer interface {
		RefreshResInfo(context.Context) (resource.Infos, error)
	}
	paths, err := sel.MustExpand()
	if err != nil {
		return data, err
	}
	var errs error
	ctx := context.Background()
	for _, path := range paths {
		obj, err := object.New(path)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		var infos resource.Infos
		if t.Refresh {
			i, ok := obj.(refreshResInfoer)
			if !ok {
				continue
			}
			infos, err = i.RefreshResInfo(ctx)
		} else {
			i, ok := obj.(loadResInfoer)
			if !ok {
				continue
			}
			infos, err = i.LoadResInfo()
		}
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		more := resourceInfosToAPI(infos, path.String(), hostname.Hostname())
		data.Items = append(data.Items, more.Items...)
	}
	return data, errs
}

func (t *CmdObjectInstanceResourceInfo) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	data, err := t.extractLocal(mergedSelector)
	if err != nil {
		return err
	}
	output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,RID:rid,KEY:key,VALUE:value",
		Output:        t.Output,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
