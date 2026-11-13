package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

// resourceInfoTimeout bounds a listing of the resource info of objects.
const resourceInfoTimeout = 5 * time.Second

// ObjectResourceInfoRefreshRemote returns the remote function asking the node
// to recompute the resource info cache of its instance. Reporting the
// refreshed key-values to the collector is the collector speaker's job,
// driven by the InstanceResourceInfoUpdated signal.
func ObjectResourceInfoRefreshRemote(rid string) func(context.Context, naming.Path, string) (any, error) {
	return func(ctx context.Context, p naming.Path, nodename string) (any, error) {
		c, err := client.New()
		if err != nil {
			return nil, err
		}
		sessionID := xsession.SessionID().UUID()
		params := api.PostInstanceActionInfoParams{SessionID: &sessionID, Rid: ptrOrNil(rid)}
		resp, err := c.PostInstanceActionInfoWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
		}
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, resp.Body))
		}
		return *resp.JSON200, nil
	}
}

// ObjectResourceInfoList lists the key-values the resources of the selected
// objects report, of the rid selected if any: of the instances of the nodes
// selected, or of all their instances without a node selector.
func ObjectResourceInfoList(objectSelector, nodeSelector, rid string, opts OptsGlobal) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	paths, err := objectselector.New(objectSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}
	var nodes []string
	if nodeSelector != "" {
		if nodes, err = nodeselector.New(nodeSelector, nodeselector.WithClient(c)).Expand(); err != nil {
			return err
		}
		if len(nodes) == 0 {
			return fmt.Errorf("no node matching %s", nodeSelector)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), resourceInfoTimeout)
	defer cancel()
	var (
		mu   sync.Mutex
		wg   sync.WaitGroup
		errs error
		l    = make(api.ResourceInfoItems, 0)
	)
	add := func(items api.ResourceInfoItems, err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = errors.Join(errs, err)
		l = append(l, filterResourceInfoItems(items, rid)...)
	}
	for _, p := range paths {
		if nodes == nil {
			wg.Add(1)
			go func(p naming.Path) {
				defer wg.Done()
				add(objectResourceInfo(ctx, c, p))
			}(p)
			continue
		}
		for _, node := range nodes {
			wg.Add(1)
			go func(p naming.Path, node string) {
				defer wg.Done()
				add(instanceResourceInfo(ctx, c, p, node))
			}(p, node)
		}
	}
	wg.Wait()
	return errors.Join(errs, output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,RID:rid,KEY:key,VALUE:value",
		Output:        opts.Output,
		Sort:          opts.Sort,
		Color:         opts.Color,
		Data:          api.ResourceInfoList{Items: l, Kind: "ResourceInfoList"},
		Colorize:      rawconfig.Colorize,
	}.Print())
}

func objectResourceInfo(ctx context.Context, c *client.T, p naming.Path) (api.ResourceInfoItems, error) {
	resp, err := c.GetObjectResourceInfoWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("%s: %s", p, resp.Status())
	}
	return resp.JSON200.Items, nil
}

// instanceResourceInfo returns the key-values of the instance of the object
// on the node, none when the node has no instance of it.
func instanceResourceInfo(ctx context.Context, c *client.T, p naming.Path, nodename string) (api.ResourceInfoItems, error) {
	resp, err := c.GetInstanceResourceInfoWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name)
	if err != nil {
		return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
	}
	switch {
	case resp.JSON200 != nil:
		return resp.JSON200.Items, nil
	case resp.StatusCode() == http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, resp.Body))
	}
}

// filterResourceInfoItems keeps the key-values of the selected rids only. The
// cache holds every rid, so the group commands and the PATTERN args filter
// on report.
func filterResourceInfoItems(items api.ResourceInfoItems, rid string) api.ResourceInfoItems {
	if rid == "" {
		return items
	}
	filtered := make(api.ResourceInfoItems, 0, len(items))
	for _, item := range items {
		for _, e := range strings.Split(rid, ",") {
			if resourceid.Match(item.Rid, e) {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return filtered
}
