package object

import (
	"context"
	"fmt"
	"strings"

	"github.com/ybbus/jsonrpc"

	"github.com/opensvc/om3/v3/core/array"
)

type (
	// collectorCaller is the part of the collector client this needs, so a
	// test can watch what an array pushes without a collector to push to.
	collectorCaller interface {
		Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error)
	}

	// ArrayPush is what came of pushing one array to the collector.
	ArrayPush struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Keys  int    `json:"keys"`
		Error string `json:"error,omitempty"`
	}
)

// PushArrays reports the configuration of the arrays of the cluster to the
// collector.
//
// The collector is fed through the jsonrpc method it exports for the type of
// each array, "update_freenas" for a freenas and so on, with the section names
// and their contents. That is the api v2 fed.
//
// It is the jsonrpc api and not a rest one because oc3, which takes the roles
// of the old collector over one at a time, exposes no array feed yet: its
// feeder serves the node, the instance and the object, and nothing about an
// array. oc3 will drop the jsonrpc api when it has taken everything over, so
// this call is to be moved to the rest feed the day oc3 grows one, and a rest
// endpoint is to be preferred over jsonrpc wherever both exist.
//
// One array failing does not stop the others: an array that is down or whose
// credentials have expired must not keep the ones that are reachable from
// being reported. What failed is in the result, and in the error.
func (t Node) PushArrays(ctx context.Context, name string) ([]ArrayPush, error) {
	client, err := t.CollectorFeedClient()
	if err != nil {
		return nil, err
	}
	// The name is the section, with or without its "array#" prefix, because
	// both spellings reach here: the scheduler names the section it read the
	// schedule from, an operator usually names the array alone. Node.Array
	// accepts both already.
	name = strings.TrimPrefix(name, "array#")

	l := make([]ArrayPush, 0)
	var errs error
	for _, item := range t.ListArrays() {
		if name != "" && item.Name != name {
			continue
		}
		push := ArrayPush{Name: item.Name, Type: item.Type}
		if err := t.pushArray(ctx, client, item, &push); err != nil {
			push.Error = err.Error()
			t.Log().Attr("array", item.Name).Warnf("push array %s: %s", item.Name, err)
			errs = fmt.Errorf("%w", err)
		}
		l = append(l, push)
	}
	if name != "" && len(l) == 0 {
		return l, fmt.Errorf("no array found matching %s in the node or cluster config", name)
	}
	return l, errs
}

// pushArray reports one array.
func (t Node) pushArray(ctx context.Context, client collectorCaller, item ArrayItem, push *ArrayPush) error {
	drv := t.Array(item.Name)
	if drv == nil {
		return fmt.Errorf("no array driver found matching type %s", item.Type)
	}
	return t.pushArrayWithDriver(ctx, client, drv, item, push)
}

// pushArrayWithDriver reports one array through a driver already resolved.
func (t Node) pushArrayWithDriver(ctx context.Context, client collectorCaller, drv array.Driver, item ArrayItem, push *ArrayPush) error {
	reporter, ok := drv.(array.Reporter)
	if !ok {
		return fmt.Errorf("the %s driver does not report its configuration", item.Type)
	}
	data, err := array.Collect(ctx, reporter)
	if err != nil {
		return err
	}
	push.Keys = len(data.Keys)
	response, err := client.Call(array.ReportMethod(item.Type), item.Name, data.Keys, data.Values)
	if err != nil {
		return err
	}
	if response != nil && response.Error != nil {
		return fmt.Errorf("%s: %s", array.ReportMethod(item.Type), response.Error.Message)
	}
	return nil
}
