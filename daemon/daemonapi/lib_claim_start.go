package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// startClaimed is how many instances of an object its claims count as
// started: one for a failover object, its flex target for a flex one.
func startClaimed(cfg *instance.Config) int {
	if cfg.Topology == topology.Flex && cfg.Flex != nil {
		return cfg.Flex.Target
	}
	return 1
}

// refuseStartOverClaim stops an instance start that runs more instances of
// the object than its namespace claims of the cpu and memory counted for it.
//
// An object claims what its processes are capped to on the instances it may
// run at once, which is the placement the orchestration keeps it to. An
// instance started on top of those is one the claim never counted, so a
// namespace user asking for it is refused. Root is not rationed: its start is
// let through, and said, since it takes the namespace over its claim.
func refuseStartOverClaim(ctx echo.Context, p naming.Path, nodename string) (bool, error) {
	if p.Namespace == naming.NsRoot {
		return true, nil
	}
	claimed := false
	for _, claimType := range claim.ComputeTypes {
		if _, ok, err := claim.ComputeLimit(p.Namespace, claimType); err == nil && ok {
			claimed = true
		}
	}
	if !claimed {
		return true, nil
	}
	var cfg *instance.Config
	for _, c := range instance.ConfigData.GetByPath(p) {
		if c.ActorConfig != nil {
			cfg = c
			break
		}
	}
	if cfg == nil {
		return true, nil
	}
	allowed := startClaimed(cfg)
	started := 0
	for node, st := range instance.StatusData.GetByPath(p) {
		if node == nodename {
			// Starting what is started takes nothing more.
			if st.Avail == status.Up || st.Avail == status.Warn {
				return true, nil
			}
			continue
		}
		if st.Avail == status.Up || st.Avail == status.Warn {
			started++
		}
	}
	if started < allowed {
		return true, nil
	}
	msg := fmt.Sprintf("the %s namespace claims the compute of %d started instance(s) of %s, and %d already run(s)", p.Namespace, allowed, p, started)
	if grantsFromContext(ctx).HasGrant(rbac.GrantRoot) {
		LogHandler(ctx, "refuseStartOverClaim").Warnf("%s: started on %s anyway, by root", msg, nodename)
		return true, nil
	}
	return false, JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "%s: %s", ErrClaimOverrun, msg)
}
