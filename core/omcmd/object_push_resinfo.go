package omcmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdObjectPushResourceInfo struct {
		OptsGlobal
		OptsLock
		NodeSelector string
	}
)

func (t *CmdObjectPushResourceInfo) doLocal(selector string) (api.ResourceInfoList, error) {
	data := api.ResourceInfoList{
		Kind: "ResourceInfoList",
	}
	sel := objectselector.New(
		selector,
		objectselector.WithLocal(true),
	)
	type pushResInfoer interface {
		PushResInfo(context.Context) (resource.Infos, error)
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
		i, ok := obj.(pushResInfoer)
		if !ok {
			continue
		}
		infos, err := i.PushResInfo(ctx)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		more := resourceInfosToAPI(infos, path.String(), hostname.Hostname())
		data.Items = append(data.Items, more.Items...)
	}
	return data, errs
}

func (t *CmdObjectPushResourceInfo) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	data, err := t.doLocal(mergedSelector)
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
