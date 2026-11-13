package omcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/actioncontext"

	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	CmdObjectInstanceResourceInfo struct {
		OptsGlobal
		commoncmd.OptsLock
		commoncmd.OptsResourceSelector
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
			infos, err = i.RefreshResInfo(t.withResourceSelector(ctx))
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
		more := resourceInfosToAPI(t.filterResInfo(infos), path.String(), hostname.Hostname())
		data.Items = append(data.Items, more.Items...)
	}
	return data, errs
}

// withResourceSelector puts the resource selector in the context the refresh
// runs with, so it only recomputes the selected resources.
func (t *CmdObjectInstanceResourceInfo) withResourceSelector(ctx context.Context) context.Context {
	if t.RID != "" {
		ctx = actioncontext.WithRID(ctx, t.RID)
	}
	if t.Subset != "" {
		ctx = actioncontext.WithSubset(ctx, t.Subset)
	}
	if t.Tag != "" {
		ctx = actioncontext.WithTag(ctx, t.Tag)
	}
	return ctx
}

// filterResInfo keeps the key-values of the selected rids only. The cache holds
// every rid, so the group commands and the PATTERN args filter on report.
func (t *CmdObjectInstanceResourceInfo) filterResInfo(infos resource.Infos) resource.Infos {
	if t.RID == "" {
		return infos
	}
	filtered := resource.NewInfos(infos.ObjectPath)
	for _, info := range infos.Resources {
		for _, e := range strings.Split(t.RID, ",") {
			if resourceid.Match(info.RID, e) {
				filtered.Resources = append(filtered.Resources, info)
				break
			}
		}
	}
	return filtered
}

// Run lists the key-values the resources of the local instances report, or
// with a node selector the ones of the instances of the nodes selected,
// through the api. With --refresh, the cache is computed again first, on
// this node, or by the nodes selected, which the command then lists the
// execs of.
func (t *CmdObjectInstanceResourceInfo) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.NodeSelector != "" {
		if t.Refresh {
			return objectaction.New(
				objectaction.WithObjectSelector(mergedSelector),
				objectaction.WithOutput(t.Output),
				objectaction.WithSort(t.Sort),
				objectaction.WithColor(t.Color),
				objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
				objectaction.WithRemoteNodes(t.NodeSelector),
				objectaction.WithRemoteFunc(commoncmd.ObjectResourceInfoRefreshRemote(t.RID)),
			).Do()
		}
		return commoncmd.ObjectResourceInfoList(mergedSelector, t.NodeSelector, t.RID, t.OptsGlobal.OptsGlobal)
	}
	data, err := t.extractLocal(mergedSelector)
	if err != nil {
		return err
	}
	return output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,RID:rid,KEY:key,VALUE:value",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          data,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
