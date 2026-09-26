package daemonapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/api"
)

// pgSetDefaultWait is how long a pg set waits for the configuration to reach
// the nodes of the object when the request does not say: the caps are applied
// by the nodes holding it, so the wait is not optional.
const pgSetDefaultWait = "30s"

// PostObjectActionPGSet sets the cgroup caps of an object, and applies them
// to its running instances without a restart.
//
// The caps are written as the pg_* keywords of the configuration, through
// the same write as a configuration update: the rbac policy, the validation
// and the claim checks weigh it as they weigh any write, and what the
// configuration holds is what every later start, failover and reboot applies.
// A cap written anywhere else would be lost at the next of them, and would
// hold nothing a claim could count.
//
// The write is waited for until it has reached every live node of the
// object, and each node running an instance is then asked to apply the
// configuration of that write, which it refuses to do on an older one.
func (a *DaemonAPI) PostObjectActionPGSet(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectActionPGSetParams) error {
	log := LogHandler(ctx, "PostObjectActionPGSet")
	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	switch kind {
	case naming.KindSvc, naming.KindVol:
	default:
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "a %s has no process group caps", kind)
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	var payload api.PostObjectActionPGSet
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	sets, err := pgSetOps(payload.Caps)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "%s", err)
	}

	// The configuration is written by a node holding it, as the user: the
	// write is weighed with their grants, as a configuration update is.
	var writer string
	for nodename := range instance.ConfigData.GetByPath(p) {
		if nodename == a.localhost || writer == "" {
			writer = nodename
		}
	}
	if writer == "" {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "object not found: %s", p)
	}
	wait := pgSetDefaultWait
	if params.Wait != nil && *params.Wait != "" {
		wait = *params.Wait
	}
	c, err := a.newProxyClient(ctx, writer, client.WithTimeout(0))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", writer, err)
	}
	resp, err := c.PatchObjectConfigWithResponse(ctx.Request().Context(), namespace, kind, name, &api.PatchObjectConfigParams{
		Set:  &sets,
		Wait: &wait,
	})
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Update config", "%s: %s", writer, err)
	}
	if resp.StatusCode() != http.StatusOK || resp.JSON200 == nil {
		// Refused, or still propagating: the answer of the write is the
		// answer, and no instance is asked anything.
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, resp.HTTPResponse.Header.Get(api.HeaderLastModified))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Update config", "the write answered no timestamp: %s", err)
	}
	ctx.Response().Header().Add(api.HeaderLastModified, updatedAt.Format(time.RFC3339Nano))
	result := api.PGSet{
		IsChanged:       resp.JSON200.IsChanged,
		ConfigUpdatedAt: &updatedAt,
		Instances:       a.pgSetApply(ctx, p, updatedAt),
	}
	log.Infof("pg set %s on %s: %s", p, strings.Join(sets, " "), describePGSetInstances(result.Instances))
	return ctx.JSON(http.StatusOK, result)
}

// pgSetApply asks every node running an instance of the object to apply the
// configuration of updatedAt, and says what each answered.
func (a *DaemonAPI) pgSetApply(ctx echo.Context, p naming.Path, updatedAt time.Time) []api.PGSetInstance {
	nodes := make([]string, 0)
	running := make(map[string]bool)
	for nodename, st := range instance.StatusData.GetByPath(p) {
		nodes = append(nodes, nodename)
		running[nodename] = st.Avail == status.Up || st.Avail == status.Warn
	}
	sort.Strings(nodes)
	l := make([]api.PGSetInstance, 0, len(nodes))
	for _, nodename := range nodes {
		item := api.PGSetInstance{Node: nodename}
		if !running[nodename] {
			item.State = api.Skipped
			l = append(l, item)
			continue
		}
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			item.State = api.Failed
			item.Reason = pgSetReason(err.Error())
			l = append(l, item)
			continue
		}
		at := updatedAt
		resp, err := c.PostInstanceActionPGUpdateWithResponse(ctx.Request().Context(), nodename, p.Namespace, p.Kind, p.Name, &api.PostInstanceActionPGUpdateParams{
			ConfigUpdatedAt: &at,
		})
		switch {
		case err != nil:
			item.State = api.Failed
			item.Reason = pgSetReason(err.Error())
		case resp.JSON200 != nil:
			item.State = api.Accepted
			item.ExecId = &resp.JSON200.ExecID
			item.SessionId = &resp.JSON200.SessionID
		default:
			item.State = api.Failed
			item.Reason = pgSetReason(fmt.Sprintf("%s: %s", resp.Status(), strings.TrimSpace(string(resp.Body))))
		}
		l = append(l, item)
	}
	return l
}

// pgSetOps turns the caps of a pg set into the keyword operations of a
// configuration update.
func pgSetOps(caps map[string]api.PGCaps) ([]string, error) {
	if len(caps) == 0 {
		return nil, fmt.Errorf("no caps to set")
	}
	sections := make([]string, 0, len(caps))
	for section := range caps {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	l := make([]string, 0)
	for _, section := range sections {
		if err := validPGSetSection(section); err != nil {
			return nil, err
		}
		c := caps[section]
		for _, e := range []struct {
			option string
			value  *string
		}{
			{"pg_cpus", c.Cpus},
			{"pg_mems", c.Mems},
			{"pg_cpu_shares", c.CpuShares},
			{"pg_cpu_quota", c.CpuQuota},
			{"pg_cpu_burst", c.CpuBurst},
			{"pg_mem_limit", c.MemLimit},
			{"pg_mem_high", c.MemHigh},
			{"pg_vmem_limit", c.VmemLimit},
			{"pg_pids_max", c.PidsMax},
			{"pg_blkio_weight", c.BlkioWeight},
		} {
			if e.value == nil {
				continue
			}
			if strings.ContainsAny(*e.value, "\n\r") {
				return nil, fmt.Errorf("%s.%s: a cap is one line", section, e.option)
			}
			l = append(l, fmt.Sprintf("%s.%s=%s", section, e.option, *e.value))
		}
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("no caps to set")
	}
	return l, nil
}

// validPGSetSection refuses a section a cap cannot be written in: the object,
// a subset and a resource hold caps, and nothing else does.
func validPGSetSection(section string) error {
	switch {
	case section == "DEFAULT":
		return nil
	case strings.HasPrefix(section, "subset#") && len(section) > len("subset#"):
		return nil
	}
	if _, err := resourceid.Parse(section); err != nil || strings.ContainsAny(section, " .=") {
		return fmt.Errorf("%s is not a section holding caps: name DEFAULT, subset#<name> or a resource id", section)
	}
	return nil
}

func pgSetReason(s string) *string {
	return &s
}

func describePGSetInstances(l []api.PGSetInstance) string {
	parts := make([]string, 0, len(l))
	for _, e := range l {
		parts = append(parts, e.Node+" "+string(e.State))
	}
	return strings.Join(parts, ", ")
}
