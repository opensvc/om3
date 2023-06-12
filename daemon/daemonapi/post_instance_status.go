package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
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
		Avail:       status.Parse(string(payloadStatus.Avail)),
		FrozenAt:    payloadStatus.FrozenAt,
		Kind:        kind.New(payloadStatus.Kind),
		Overall:     status.Parse(string(payloadStatus.Overall)),
		StatusGroup: nil,
		UpdatedAt:   payloadStatus.UpdatedAt,
	}
	if payloadStatus.App != nil {
		instanceStatus.App = *payloadStatus.App
	}
	if payloadStatus.Children != nil {
		relation := toPathRelationL(payloadStatus.Children)
		if len(relation) > 0 {
			instanceStatus.Children = relation
		}
	}
	if payloadStatus.Constraints != nil {
		instanceStatus.Constraints = *payloadStatus.Constraints
	}
	if payloadStatus.Csum != nil {
		instanceStatus.Csum = *payloadStatus.Csum
	}
	if payloadStatus.Drp != nil {
		instanceStatus.DRP = *payloadStatus.Drp
	}
	if payloadStatus.Env != nil {
		instanceStatus.Env = *payloadStatus.Env
	}
	if payloadStatus.Optional != nil {
		instanceStatus.Optional = status.Parse(string(*payloadStatus.Optional))
	}
	if payloadStatus.Parents != nil {
		relation := toPathRelationL(payloadStatus.Parents)
		if len(relation) > 0 {
			instanceStatus.Parents = relation
		}
	}
	if payloadStatus.Preserved != nil {
		instanceStatus.Preserved = *payloadStatus.Preserved
	}
	if prov, err := provisioned.NewFromString(string(payloadStatus.Provisioned)); err != nil {
		return nil, err
	} else {
		instanceStatus.Provisioned = prov
	}
	if payloadStatus.Resources != nil {
		resources := make([]resource.ExposedStatus, 0)
		for _, v := range *payloadStatus.Resources {
			exposed := resource.ExposedStatus{
				Rid:    v.Rid,
				Label:  v.Label,
				Status: status.Parse(string(v.Status)),
				Type:   v.Type,
			}
			if v.Disable != nil {
				exposed.Disable = resource.DisableFlag(*v.Disable)
			}
			if v.Encap != nil {
				exposed.Encap = resource.EncapFlag(*v.Encap)
			}
			if v.Info != nil {
				info := make(map[string]interface{})
				for n, value := range *v.Info {
					info[n] = value
				}
				exposed.Info = info
			}
			if v.Log != nil {
				l := make([]*resource.StatusLogEntry, 0)
				for _, logEntry := range *v.Log {
					l = append(l, &resource.StatusLogEntry{
						Level:   resource.Level(logEntry.Level),
						Message: logEntry.Message,
					})
				}
				exposed.Log = l
			}
			if v.Monitor != nil {
				exposed.Monitor = resource.MonitorFlag(*v.Monitor)
			}
			if v.Optional != nil {
				exposed.Optional = resource.OptionalFlag(*v.Optional)
			}
			if v.Provisioned != nil {
				resProv := resource.ProvisionStatus{}
				if provState, err := provisioned.NewFromString(string(v.Provisioned.State)); err != nil {
					return nil, err
				} else {
					resProv.State = provState
				}
				if v.Provisioned.Mtime != nil {
					resProv.Mtime = *v.Provisioned.Mtime
				}
				exposed.Provisioned = resProv

			}
			if v.Restart != nil {
				exposed.Restart = resource.RestartFlag(*v.Restart)
			}
			if rid, err := resourceid.Parse(v.Rid); err == nil {
				exposed.ResourceID = rid
			}
			if v.Standby != nil {
				exposed.Standby = resource.StandbyFlag(*v.Standby)
			}
			if v.Subset != nil {
				exposed.Subset = *v.Subset
			}
			if v.Tags != nil {
				exposed.Tags = *v.Tags
			}
			resources = append(resources, exposed)
		}
		instanceStatus.Resources = resources
	}
	if payloadStatus.Running != nil {
		instanceStatus.Running = append([]string{}, *payloadStatus.Running...)
	}
	if payloadStatus.Slaves != nil {
		relation := toPathRelationL(payloadStatus.Slaves)
		if len(relation) > 0 {
			instanceStatus.Slaves = relation
		}
	}
	if payloadStatus.Subsets != nil {
		subSets := make(map[string]instance.SubsetStatus)
		for rid, s := range *payloadStatus.Subsets {
			subSets[rid] = instance.SubsetStatus{
				Parallel: s.Parallel,
			}
		}
		instanceStatus.Subsets = subSets
	}
	return &instanceStatus, nil
}

func toPathRelationL(p *api.PathRelation) []path.Relation {
	nv := make([]path.Relation, 0)
	for _, v := range *p {
		nv = append(nv, path.Relation(v))
	}
	return nv
}
