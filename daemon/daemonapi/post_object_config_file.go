package daemonapi

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/tombmon"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) PostObjectConfigFile(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostObjectConfigFileParams) error {
	log := LogHandler(ctx, "PostObjectConfigFile")

	// A namespace configuration is the namespace itself, so creating one is
	// creating the namespace, whatever admin of it anyone holds.
	if kind == naming.KindNscfg {
		if v, err := assertNamespaceConfigWriter(ctx); !v {
			return err
		}
	} else if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request path", "%s", err)
	}
	// Read before anything is checked against it: the write lands only
	// over this file.
	base, err := readConfigBase(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read config", "%s", err)
	}

	// Forced, the configuration of an object that exists is replaced as a
	// creation writes it, with the checks of an update. An object exists
	// from the moment its configuration file is written, before the daemon
	// knows it: two creates racing, the second is not one.
	force := params.Force != nil && *params.Force
	// A tombstone of the path with pending holders: nodes hold the
	// instance of an object purged or deleted elsewhere, and a new object
	// of the name would meet theirs. Forced, the tombstone is dropped once
	// the configuration is written: a create refused or failed leaves it,
	// and a crash between the two leaves it pending, the safe side, a
	// create --force again dropping it.
	pending := tombstone.Data.PendingHolders(p.String(), "")
	if len(pending) > 0 && !force {
		if live := tombstone.Data.PendingHolders(p.String(), tombstone.HolderLive); len(live) > 0 {
			return JSONProblemf(ctx, http.StatusConflict, "Conflict", "%s is being deleted on %s: retry once done, a create forced meanwhile having its new instances join that delete", p, strings.Join(live, ", "))
		}
		return JSONProblemf(ctx, http.StatusConflict, "Conflict", "%s was purged or deleted with %s away, holding it still: purge it there, or create with --force to drop its tombstone", p, strings.Join(pending, ", "))
	}
	exists := len(instance.MonitorData.GetNodesByPath(p)) > 0 || base.exists()
	if exists && !force {
		return JSONProblemf(ctx, http.StatusConflict, "Conflict", "Use the PUT method instead of POST to update the object config")
	}

	// Read and parse the config body to validate RBAC rules
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}

	// The configuration written is the one the checks judge: without what
	// the configuration it was copied from recorded of itself, unless
	// restored.
	restore := params.Restore != nil && *params.Restore
	var sets []keyop.T
	if params.Set != nil {
		sets = keyop.ParseOps(*params.Set)
	}
	var src *storeSource
	if params.Store != nil || params.Snapshot != nil {
		if len(body) > 0 {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "a create from a store takes the configuration of the snapshot, and no body")
		}
		var status int
		if src, status, err = openStoreSource(ctx, p, params.Store, params.Snapshot); err != nil {
			return JSONProblemf(ctx, status, http.StatusText(status), "%s", err)
		}
		defer src.close()
		body = src.config
	}
	oc, body, err := object.CreatedConfig(p, body, restore, sets...)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad config", "%s", err)
	}

	// A dry run runs every check, and answers what they found.
	dryRun := params.DryRun != nil && *params.DryRun
	check := a.newConfigCheck(ctx, p, dryRun)
	// Forced, the tombstones of the path are dropped as they are once the
	// configuration is written, one recorded by a delete since the check
	// above included.
	if force {
		check.afterCommit = func() error {
			dropped, err := tombmon.Drop(p.String())
			if err != nil {
				return fmt.Errorf("%s is created, and its tombstone not dropped: %w: create it again with --force", p, err)
			}
			if len(dropped) > 0 {
				log.Infof("%s: tombstone dropped, holders %s", p, dropped)
			}
			return nil
		}
	}

	// Validate RBAC rules for config keys
	if !check.rbac(body) {
		log.Warnf("%s: rbac: %s", p, check.refusal.err)
		return check.answer()
	}

	// The keys the configuration references, and the snapshot its
	// provision restores from, come once the configuration passed: the
	// requester may read the snapshot, and the keys land with the
	// configuration in a namespace it is an administrator of. They are
	// copied as the configuration lands, which a dry run does not reach:
	// it judges the configuration the create would write, the snapshot
	// read, and writes no key. A dry run the keyword policy refused judges
	// the configuration without them.
	if src != nil && !check.refused() {
		if oc, body, base, err = src.apply(ctx, p, body, base); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Create from store", "%s", err)
		}
	}

	if exists && !check.stopToChange(body, force) {
		return check.answer()
	}

	// Flag the instance stopped on purpose if the daemon would start it on
	// its own, so it does not do so before the operator asked for anything.
	var flagStopped bool
	switch oc.Config().GetString(key.Parse("orchestrate")) {
	case "ha", "start":
		flagStopped = true
	}
	waitLocal := params.WaitLocal != nil && *params.WaitLocal
	return a.writeObjectConfigFile(ctx, check, body, base, params.Wait, waitLocal, force, flagStopped)
}
