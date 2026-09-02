package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectaction"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	CmdObjectInstanceResourceInfo struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsResourceSelector
		NodeSelector string

		// Refresh recomputes the resource info cache before reporting it.
		// Without it, the cached key-values are reported as-is.
		Refresh bool
	}
)

func (t *CmdObjectInstanceResourceInfo) Run(kind string) error {
	if t.Refresh {
		return t.refresh(kind)
	}
	return t.list(kind)
}

// refresh asks the nodes holding an instance to recompute their resource info
// cache. Reporting the refreshed key-values to the collector is the collector
// speaker's job, driven by the InstanceResourceInfoUpdated signal.
func (t *CmdObjectInstanceResourceInfo) refresh(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	return objectaction.New(
		objectaction.WithObjectSelector(mergedSelector),
		objectaction.WithOutput(t.Output),
		objectaction.WithColor(t.Color),
		objectaction.WithIgnoreNotFound(t.IgnoreNotFound),
		objectaction.WithAsyncTime(t.Time),
		objectaction.WithAsyncWait(t.Wait),
		objectaction.WithAsyncWatch(t.Watch),
		objectaction.WithRemoteNodes(t.NodeSelector),
		objectaction.WithRemoteFunc(func(ctx context.Context, p naming.Path, nodename string) (interface{}, error) {
			c, err := client.New()
			if err != nil {
				return nil, err
			}
			params := api.PostInstanceActionInfoParams{}
			{
				sid := xsession.Sid().UUID()
				params.SessionId = &sid
			}
			if t.RID != "" {
				rid := t.RID
				params.Rid = &rid
			}
			response, err := c.PostInstanceActionInfoWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
			if err != nil {
				return nil, err
			}
			switch {
			case response.JSON200 != nil:
				return *response.JSON200, nil
			case response.JSON401 != nil:
				return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON401)
			case response.JSON403 != nil:
				return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON403)
			case response.JSON500 != nil:
				return nil, fmt.Errorf("%s: node %s: %s", p, nodename, *response.JSON500)
			default:
				return nil, fmt.Errorf("%s: node %s: unexpected response: %s", p, nodename, response.Status())
			}
		}),
	).Do()
}

// filterItems keeps the key-values of the selected rids only. The cache holds
// every rid, so the group commands and the PATTERN args filter on report.
func (t *CmdObjectInstanceResourceInfo) filterItems(items api.ResourceInfoItems) api.ResourceInfoItems {
	if t.RID == "" {
		return items
	}
	filtered := make(api.ResourceInfoItems, 0, len(items))
	for _, item := range items {
		for _, e := range strings.Split(t.RID, ",") {
			if resourceid.Match(item.Rid, e) {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}

func (t *CmdObjectInstanceResourceInfo) list(kind string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	l := make(api.ResourceInfoItems, 0)
	q := make(chan api.ResourceInfoItems)
	errC := make(chan error)
	doneC := make(chan string)
	todoP := len(paths)
	for _, path := range paths {
		go func(p naming.Path) {
			defer func() { doneC <- p.String() }()
			response, err := c.GetObjectResourceInfoWithResponse(ctx, p.Namespace, p.Kind, p.Name)
			if err != nil {
				errC <- err
				return
			}
			switch {
			case response.JSON200 != nil:
				q <- t.filterItems(response.JSON200.Items)
			case response.JSON401 != nil:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON401)
			case response.JSON403 != nil:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON403)
			case response.JSON500 != nil:
				errC <- fmt.Errorf("%s: %s", p, *response.JSON500)
			default:
				errC <- fmt.Errorf("%s: unexpected response: %s", p, response.Status())
			}
		}(path)
	}

	var (
		errs  error
		doneP int
	)

	for {
		select {
		case err := <-errC:
			errs = errors.Join(errs, err)
		case items := <-q:
			l = append(l, items...)
		case <-doneC:

			if !(doneP == todoP) {
				doneP++
			}

			if doneP == todoP {
				goto out
			}

		case <-ctx.Done():
			errs = errors.Join(errs, ctx.Err())
			goto out
		}
	}

out:

	output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,RID:rid,KEY:key,VALUE:value",
		Output:        t.Output,
		Color:         t.Color,
		Data:          api.ResourceInfoList{Items: l, Kind: "ResourceInfoList"},
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errs
}
