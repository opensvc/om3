package daemonapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/pool"
	"github.com/opensvc/om3/v3/daemon/api"
)

// computeClaimGrants is what this node has let namespaces take of each
// compute type and has not seen claimed yet, by type. It is consulted only on
// the node speaking for the cluster, which is where every claim is answered.
var computeClaimGrants = func() map[string]*pool.Grants {
	m := make(map[string]*pool.Grants)
	for _, claimType := range claim.ComputeTypes {
		m[claimType] = pool.NewGrants(claimGrantTTL).WithFormat(func(v int64) string {
			return claim.Format(claimType, v)
		})
	}
	return m
}()

// PostComputeClaim answers whether a namespace may have an object claim what
// it asks of the cpu and memory, and counts the answer until the
// configuration saying so is seen.
//
// It is answered on the node speaking for the cluster, for the reason a pool
// claim is: what a namespace holds is read from the configurations the
// cluster shares, which a write reaches a moment after it is made, and claims
// answered from that reading alone all fit where together they do not.
//
// What each object claims is read from the instance configurations this node
// holds of every node, so answering asks no peer anything.
func (a *DaemonAPI) PostComputeClaim(ctx echo.Context) error {
	var payload api.PostComputeClaim
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	if payload.Namespace == "" || payload.Path == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "the namespace and the object a claim is about are both needed")
	}
	if v, err := assertAdmin(ctx, payload.Namespace); !v {
		return err
	}
	speaker := speakerNode()
	if speaker != "" && speaker != a.localhost {
		return a.proxy(ctx, speaker, func(c *client.T) (*http.Response, error) {
			return c.PostComputeClaim(ctx.Request().Context(), payload)
		})
	}
	type asked struct {
		claimType string
		to, limit int64
		held      map[string]int64
	}
	l := make([]asked, 0, len(claim.ComputeTypes))
	for _, claimType := range claim.ComputeTypes {
		to, ok := payload.Claims[claimType]
		if !ok {
			continue
		}
		limit, capped, err := claim.ComputeLimit(payload.Namespace, claimType)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Claim limit", "%s", err)
		}
		if !capped {
			continue
		}
		if to == claim.Unbounded {
			why := fmt.Sprintf("the %s namespace claims the %s, and %s runs a process capped by nothing on it: cap it with %s on its resource or its object (the default of a claim caps the containers only)", payload.Namespace, claimType, payload.Path, capOptionOf(claimType))
			return ctx.JSON(http.StatusOK, api.ComputeClaim{Granted: false, Reason: &why})
		}
		held, unbounded := computeClaimHeldByPath(payload.Namespace, claimType)
		delete(unbounded, payload.Path)
		if len(unbounded) > 0 {
			why := fmt.Sprintf("the %s namespace claims the %s, and runs processes capped by nothing on it in %s", payload.Namespace, claimType, strings.Join(sortedKeys(unbounded), ", "))
			return ctx.JSON(http.StatusOK, api.ComputeClaim{Granted: false, Reason: &why})
		}
		l = append(l, asked{claimType: claimType, to: to, limit: limit, held: held})
	}
	now := time.Now()
	// Weighed first and recorded after, so a claim refused on one type
	// takes nothing of the others.
	for _, e := range l {
		if granted, why := computeClaimGrants[e.claimType].Probe(payload.Namespace, e.claimType, payload.Path, e.to, e.limit, e.held, now); !granted {
			why = e.claimType + ": " + why
			return ctx.JSON(http.StatusOK, api.ComputeClaim{Granted: false, Reason: &why})
		}
	}
	if len(l) == 0 {
		return ctx.JSON(http.StatusOK, api.ComputeClaim{Granted: true})
	}
	for _, e := range l {
		computeClaimGrants[e.claimType].Fits(payload.Namespace, e.claimType, payload.Path, e.to, e.limit, e.held, now)
	}
	expiresAt := computeClaimGrants[l[0].claimType].ExpiresAt(now)
	return ctx.JSON(http.StatusOK, api.ComputeClaim{Granted: true, ExpiresAt: &expiresAt})
}

// computeClaimHeldByPath is what each object of a namespace claims of a
// compute type, by path, and the objects claiming it without bound.
//
// Every node publishes the configuration of the objects it runs, and a
// write reaches them one after the other: an object is counted for the most
// any of them says it claims.
func computeClaimHeldByPath(namespace, claimType string) (map[string]int64, map[string]bool) {
	held := make(map[string]int64)
	unbounded := make(map[string]bool)
	for _, e := range instance.ConfigData.GetAll() {
		if e.Path.Namespace != namespace || e.Value.ActorConfig == nil {
			continue
		}
		v, ok := e.Value.Claims[claimType]
		if !ok {
			continue
		}
		p := e.Path.String()
		if v == claim.Unbounded {
			unbounded[p] = true
			continue
		}
		if v > held[p] {
			held[p] = v
		}
	}
	for p := range unbounded {
		delete(held, p)
	}
	return held, unbounded
}

// configuredComputeClaims is what the object claims according to the
// configuration this node publishes of it, and nil for an object it
// publishes none of, which is one being created.
func configuredComputeClaims(p naming.Path) map[string]int64 {
	for _, cfg := range instance.ConfigData.GetByPath(p) {
		if cfg.ActorConfig != nil && cfg.Claims != nil {
			return cfg.Claims
		}
	}
	return nil
}

func capOptionOf(claimType string) string {
	switch claimType {
	case claim.TypeCPU:
		return "pg_cpu_quota"
	default:
		return "pg_mem_limit"
	}
}

func sortedKeys(m map[string]bool) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	sort.Strings(l)
	return l
}
