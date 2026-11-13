package commoncmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodeselector"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
)

// ScheduleListOutput is the listing of the schedules of instances.
const ScheduleListOutput = "tab=OBJECT:meta.object,NODE:meta.node,ACTION:data.action,KEY:data.key,LAST_RUN_AT:data.last_run_at,NEXT_RUN_AT:data.next_run_at,SCHEDULE:data.schedule,ON:data.on[*],PACE:data.pace"

// InstanceSchedules returns the schedules of the instances of the selected
// objects on the selected nodes, each asked of its node.
func InstanceSchedules(c *client.T, objectSelector, nodeSelector string) (api.ScheduleList, error) {
	var errs error
	data := api.ScheduleList{Kind: "ScheduleList"}
	nodenames, err := nodeselector.New(nodeSelector, nodeselector.WithClient(c)).Expand()
	if err != nil {
		return data, err
	}
	paths, err := objectselector.New(objectSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		return data, err
	}
	for _, nodename := range nodenames {
		for _, path := range paths {
			if d, err := instanceSchedule(c, nodename, path); err != nil {
				errs = errors.Join(errs, err)
			} else {
				data.Items = append(data.Items, d.Items...)
			}
		}
	}
	return data, errs
}

func instanceSchedule(c *client.T, nodename string, path naming.Path) (api.ScheduleList, error) {
	resp, err := c.GetInstanceScheduleWithResponse(context.Background(), nodename, path.Namespace, path.Kind, path.Name)
	if err != nil {
		return api.ScheduleList{}, err
	}
	switch resp.StatusCode() {
	case 200:
		return *resp.JSON200, nil
	case 404:
		// No instance on the node is no schedule there. No schedule
		// table cached is an object with no schedule, as the cluster
		// configuration, or one the scheduler has not loaded yet, as
		// after the daemon started: the local listing answers.
		if resp.JSON404 != nil && resp.JSON404.Title == "No local instance" {
			return api.ScheduleList{}, nil
		}
		return api.ScheduleList{}, fmt.Errorf("%s: %s: %s", path, nodename, resp.Status())
	case 401:
		return api.ScheduleList{}, fmt.Errorf("%s: %s: %s", path, nodename, *resp.JSON401)
	case 403:
		return api.ScheduleList{}, fmt.Errorf("%s: %s: %s", path, nodename, *resp.JSON403)
	default:
		return api.ScheduleList{}, fmt.Errorf("%s: %s: unexpected statuscode: %s", path, nodename, resp.Status())
	}
}
