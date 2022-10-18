package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/guregu/null"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/placement"
	"opensvc.com/opensvc/core/priority"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/core/topology"
	"opensvc.com/opensvc/daemon/daemondata"
)

func (a *DaemonApi) PostObjectStatus(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		p       path.T
		payload PostObjectStatus
	)
	log := getLogger(r, "PostObjectStatus")
	log.Debug().Msgf("starting")
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Warn().Err(err).Msgf("decode body")
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse path: %s", payload.Path)
		sendErrorf(w, http.StatusBadRequest, "invalid path %s", payload.Path)
		return
	}
	instanceStatus, err := postObjectStatusToInstanceStatus(payload)
	if err != nil {
		log.Warn().Err(err).Msgf("can't parse instance status %s", payload.Path)
		sendError(w, http.StatusBadRequest, "can't parse instance status")
		return
	}
	dataCmd := daemondata.BusFromContext(r.Context())
	if err := daemondata.SetInstanceStatus(dataCmd, p, *instanceStatus); err != nil {
		log.Warn().Err(err).Msgf("can't set instance status for %s", p)
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func postObjectStatusToInstanceStatus(payload PostObjectStatus) (*instance.Status, error) {
	payloadStatus := payload.Status
	instanceStatus := instance.Status{
		Avail:       status.Parse(payloadStatus.Avail),
		Frozen:      payloadStatus.Frozen,
		Kind:        kind.New(payloadStatus.Kind),
		Overall:     status.Parse(payloadStatus.Overall),
		Scale:       null.Int{},
		StatusGroup: nil,
		Updated:     payloadStatus.Updated,
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
	if payloadStatus.FlexMax != nil {
		instanceStatus.FlexMax = *payloadStatus.FlexMax
	}
	if payloadStatus.FlexMin != nil {
		instanceStatus.FlexMin = *payloadStatus.FlexMin
	}
	if payloadStatus.FlexTarget != nil {
		instanceStatus.FlexTarget = *payloadStatus.FlexTarget
	}
	if payloadStatus.Optional != nil {
		instanceStatus.Optional = status.Parse(*payloadStatus.Optional)
	}
	if payloadStatus.Orchestrate != nil {
		instanceStatus.Orchestrate = string(*payloadStatus.Orchestrate)
	}
	if payloadStatus.Parents != nil {
		relation := toPathRelationL(payloadStatus.Parents)
		if len(relation) > 0 {
			instanceStatus.Parents = relation
		}
	}
	if payloadStatus.Placement != nil {
		instanceStatus.Placement = placement.New(string(*payloadStatus.Placement))
	}
	if payloadStatus.Preserved != nil {
		instanceStatus.Preserved = *payloadStatus.Preserved
	}
	if payloadStatus.Priority != nil {
		instanceStatus.Priority = priority.T(*payloadStatus.Priority)
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
				Status: status.Parse(v.Status),
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
	if payloadStatus.Topology != nil {
		instanceStatus.Topology = topology.New(string(*payloadStatus.Topology))
	}
	if payloadStatus.Subsets != nil {
		subSets := make(map[string]instance.SubsetStatus)
		for _, s := range *payloadStatus.Subsets {
			subSets[s.Rid] = instance.SubsetStatus{
				Parallel: s.Parallel,
			}
		}
		instanceStatus.Subsets = subSets
	}
	return &instanceStatus, nil
}

func toPathRelationL(p *PathRelation) []path.Relation {
	nv := make([]path.Relation, 0)
	for _, v := range *p {
		nv = append(nv, path.Relation(v))
	}
	return nv
}
