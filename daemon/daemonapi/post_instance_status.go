package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostInstanceStatus(ctx echo.Context) error {
	var (
		err     error
		p       path.T
		payload api.PostInstanceStatus
	)
	log := LogHandler(ctx, "PostInstanceStatus")
	log.Debug().Msgf("starting")
	if err := ctx.Bind(&payload); err != nil {
		log.Warn().Err(err).Msgf("decode body")
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse path: %s", payload.Path)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing path '%s': %s", payload.Path, err)
	}
	instanceStatus, err := postInstanceStatusToInstanceStatus(payload)
	if err != nil {
		log.Warn().Err(err).Msgf("Error transtyping instance status: %#v", payload)
		return JSONProblemf(ctx, http.StatusBadRequest, "Error transtyping instance status", "%s", err)
	}
	localhost := hostname.Hostname()
	a.EventBus.Pub(&msgbus.InstanceStatusPost{Path: p, Node: localhost, Value: *instanceStatus},
		pubsub.Label{"path", payload.Path},
		pubsub.Label{"node", localhost},
	)
	return ctx.JSON(http.StatusOK, nil)
}

func postInstanceStatusToInstanceStatus(payload api.PostInstanceStatus) (*instance.Status, error) {
	payloadStatus := payload.Status
	instanceStatus := instance.Status{
		Avail:         status.Parse(string(payloadStatus.Avail)),
		FrozenAt:      payloadStatus.FrozenAt,
		Overall:       status.Parse(string(payloadStatus.Overall)),
		UpdatedAt:     payloadStatus.UpdatedAt,
		LastStartedAt: payloadStatus.LastStartedAt,
	}
	instanceStatus.Constraints = payloadStatus.Constraints
	instanceStatus.Optional = status.Parse(string(payloadStatus.Optional))
	if prov, err := provisioned.NewFromString(string(payloadStatus.Provisioned)); err != nil {
		return nil, err
	} else {
		instanceStatus.Provisioned = prov
	}
	resources := make([]resource.ExposedStatus, 0)
	for _, v := range payloadStatus.Resources {
		exposed := resource.ExposedStatus{
			Rid:    v.Rid,
			Label:  v.Label,
			Status: status.Parse(string(v.Status)),
			Type:   v.Type,
		}
		exposed.Disable = resource.DisableFlag(v.Disable)
		exposed.Encap = resource.EncapFlag(v.Encap)
		info := make(map[string]interface{})
		for n, value := range v.Info {
			info[n] = value
		}
		exposed.Info = info
		l := make([]*resource.StatusLogEntry, 0)
		for _, logEntry := range v.Log {
			l = append(l, &resource.StatusLogEntry{
				Level:   resource.Level(logEntry.Level),
				Message: logEntry.Message,
			})
		}
		exposed.Log = l
		exposed.Monitor = resource.MonitorFlag(v.Monitor)
		exposed.Optional = resource.OptionalFlag(v.Optional)
		resProv := resource.ProvisionStatus{}
		if provState, err := provisioned.NewFromString(string(v.Provisioned.State)); err != nil {
			return nil, err
		} else {
			resProv.State = provState
		}
		resProv.Mtime = v.Provisioned.Mtime
		exposed.Provisioned = resProv
		exposed.Restart = resource.RestartFlag(v.Restart)
		if rid, err := resourceid.Parse(v.Rid); err == nil {
			exposed.ResourceID = rid
		}
		exposed.Standby = resource.StandbyFlag(v.Standby)
		exposed.Subset = v.Subset
		exposed.Tags = append([]string{}, v.Tags...)
		resources = append(resources, exposed)
	}
	instanceStatus.Resources = resources
	instanceStatus.Running = append([]string{}, payloadStatus.Running...)
	return &instanceStatus, nil
}
