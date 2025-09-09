package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodes(ctx echo.Context, params api.GetNodesParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
	}
	name := "GetNodes"
	log := LogHandler(ctx, name)
	if err := meta.Expand(); err != nil {
		log.Errorf("%s: %s", name, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	configs := node.ConfigData.GetAll()
	l := make(api.NodeItems, 0)
	for _, config := range configs {
		if !meta.HasNode(config.Node) {
			continue
		}
		monitor := node.MonitorData.GetByNode(config.Node)
		status := node.StatusData.GetByNode(config.Node)
		d := api.NodeItem{
			Kind: "NodeItem",
			Meta: api.NodeMeta{
				Node: config.Node,
			},
			Data: api.Node{},
		}
		if config.Value != nil {
			d.Data.Config = &api.NodeConfig{
				Env:                    config.Value.Env,
				MaintenanceGracePeriod: config.Value.MaintenanceGracePeriod,
				MaxParallel:            config.Value.MaxParallel,
				MinAvailMemPct:         config.Value.MinAvailMemPct,
				MinAvailSwapPct:        config.Value.MinAvailSwapPct,
				PRKey:                  config.Value.PRKey,
				SSHKey:                 config.Value.SSHKey,
				ReadyPeriod:            config.Value.ReadyPeriod,
				RejoinGracePeriod:      config.Value.RejoinGracePeriod,
				SplitAction:            config.Value.SplitAction,
			}
		}
		if status != nil {
			d.Data.Status = &api.NodeStatus{
				Agent:        status.Agent,
				API:          fmt.Sprint(status.API),
				Arbitrators:  make(map[string]api.ArbitratorStatus),
				Compat:       status.Compat,
				FrozenAt:     status.FrozenAt,
				Gen:          make(map[string]uint64),
				IsLeader:     status.IsLeader,
				IsOverloaded: status.IsOverloaded,
				Labels:       make(map[string]string),
			}
			for k, v := range status.Arbitrators {
				d.Data.Status.Arbitrators[k] = api.ArbitratorStatus{
					Status: api.Status(v.Status.String()),
					Url:    v.URL,
					Weight: v.Weight,
				}
			}
			for k, v := range status.Gen {
				d.Data.Status.Gen[k] = v
			}
			for k, v := range status.Labels {
				d.Data.Status.Labels[k] = v
			}
		}
		if monitor != nil {
			d.Data.Monitor = &api.NodeMonitor{
				GlobalExpect:          monitor.GlobalExpect.String(),
				GlobalExpectUpdatedAt: monitor.GlobalExpectUpdatedAt,
				LocalExpect:           monitor.LocalExpect.String(),
				LocalExpectUpdatedAt:  monitor.LocalExpectUpdatedAt,
				OrchestrationID:       monitor.OrchestrationID.String(),
				OrchestrationIsDone:   monitor.OrchestrationIsDone,
				SessionID:             monitor.SessionID.String(),
				State:                 monitor.State.String(),
				StateUpdatedAt:        monitor.StateUpdatedAt,
				UpdatedAt:             monitor.UpdatedAt,
			}
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, api.NodeList{Kind: "NodeList", Items: l})
}
