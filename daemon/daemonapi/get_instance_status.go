package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/api"
)

// GetNetworks returns network status list.
func (a *DaemonApi) GetInstanceStatus(ctx echo.Context) error {
	data := instance.StatusData.GetAll()
	l := make(api.GetInstanceStatusArray, len(data))
	for i, e := range data {
		l[i] = api.GetInstanceStatusElement{
			Meta: api.InstanceMeta{
				Node:   e.Node,
				Object: e.Path.String(),
			},
			Data: api.InstanceStatus{
				Avail:         api.Status(e.Value.Avail.String()),
				Constraints:   e.Value.Constraints,
				FrozenAt:      e.Value.FrozenAt,
				LastStartedAt: e.Value.LastStartedAt,
				Optional:      api.Status(e.Value.Optional.String()),
				Overall:       api.Status(e.Value.Overall.String()),
				Provisioned:   api.Provisioned(e.Value.Provisioned.String()),
			},
		}

		running := make([]string, 0)
		for i, d := range e.Value.Running {
			running[i] = d
		}
		l[i].Data.Running = running

		resources := make([]api.ResourceExposedStatus, len(e.Value.Resources))
		for i, d := range e.Value.Resources {
			info := make(map[string]any)
			for i, d := range d.Info {
				info[i] = d
			}
			log := make(api.ResourceLog, 0)
			for _, d := range d.Log {
				if d == nil {
					continue
				}
				log = append(log, api.ResourceLogEntry{
					Level:   string(d.Level),
					Message: d.Message,
				})
			}
			nd := api.ResourceExposedStatus{
				Disable:  bool(d.Disable),
				Encap:    bool(d.Encap),
				Info:     info,
				Label:    d.Label,
				Log:      log,
				Monitor:  bool(d.Monitor),
				Optional: bool(d.Optional),
				Provisioned: api.ResourceProvisionStatus{
					State: api.Provisioned(d.Provisioned.State.String()),
					Mtime: d.Provisioned.Mtime,
				},
				Restart: int(d.Restart),
				Rid:     d.Rid,
				Standby: bool(d.Standby),
				Status:  api.Status(d.Status.String()),
				Subset:  d.Subset,
				Tags:    append([]string{}, d.Tags...),
				Type:    d.Type,
			}
			resources[i] = nd
		}
		l[i].Data.Resources = resources
	}
	return ctx.JSON(http.StatusOK, l)
}
