package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	// InstanceDeviceLine is a device of a resource of an instance as a
	// listing shows it, with the node of the instance.
	InstanceDeviceLine struct {
		Node string `json:"node"`
		api.InstanceDevice
	}
)

// instanceDevicesOutput is the listing of the devices of instances.
const instanceDevicesOutput = "tab=OBJECT:path,NODE:node,RESOURCE:rid,DRIVER_GROUP:driver.group,DRIVER_NAME:driver.name,ROLE:role,DEVICE:device"

// InstanceDeviceList lists the devices of the resources of the instances of
// the selected objects on the selected nodes, each asked of its node.
func InstanceDeviceList(objectSelector, nodeSelector, roles string, opts OptsGlobal) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	nodes, err := nodeselector.New(nodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no node matching %s", nodeSelector)
	}
	paths, err := objectselector.New(objectSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return err
	}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		errs  error
		lines = make([]InstanceDeviceLine, 0)
	)
	for _, p := range paths {
		for _, node := range nodes {
			wg.Add(1)
			go func(p naming.Path, node string) {
				defer wg.Done()
				l, err := instanceDevices(context.Background(), c, p, node, roles)
				mu.Lock()
				defer mu.Unlock()
				errs = errors.Join(errs, err)
				lines = append(lines, l...)
			}(p, node)
		}
	}
	wg.Wait()
	return errors.Join(errs, output.Renderer{
		DefaultOutput: instanceDevicesOutput,
		Output:        opts.Output,
		Sort:          opts.Sort,
		Color:         opts.Color,
		Data: ListingData(opts.Output, NewList("InstanceDeviceList", lines), func() any {
			return lines
		}),
		Colorize: rawconfig.Colorize,
	}.Print())
}

// instanceDevices returns the devices of the instance of the object on the
// node, none when the node has no instance of it.
func instanceDevices(ctx context.Context, c *client.T, p naming.Path, nodename, roles string) ([]InstanceDeviceLine, error) {
	params := api.GetInstanceDevicesParams{Roles: ptrOrNil(roles)}
	resp, err := c.GetInstanceDevicesWithResponse(ctx, nodename, p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return nil, fmt.Errorf("%s: node %s: %w", p, nodename, err)
	}
	switch {
	case resp.JSON200 != nil:
	case resp.StatusCode() == http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("%s: %w", p, nodeResponseError(nodename, resp, resp.Body))
	}
	l := make([]InstanceDeviceLine, len(resp.JSON200.Items))
	for i, item := range resp.JSON200.Items {
		l[i] = InstanceDeviceLine{Node: item.Meta.Node, InstanceDevice: item.Data}
	}
	return l, nil
}
