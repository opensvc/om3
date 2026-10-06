package daemonapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// writeObjectConfigFile writes a whole configuration file, and with wait,
// holds the answer until it has reached every live node of the object, as a
// configuration update does.
//
// The file is written by the node the request reached, which need not be one
// of the nodes of the object, so the nodes waited for are the ones of the
// configuration written.
//
// The file is written only over the base the request was checked against,
// and the write is refused as a conflict if another landed since.
//
// With waitKnown, as a creation asks with wait_local, the answer is held
// until this daemon knows the object, so the request that follows finds it:
// the daemon learns of an object a moment after its configuration is
// written, and a provision asked in that moment was told the object did not
// exist.
func (a *DaemonAPI) writeObjectConfigFile(ctx echo.Context, p naming.Path, body []byte, base configBase, wait *api.Wait, waitKnown bool) error {
	waitCtx, cancel, waiting, err := waitContext(ctx, wait)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	defer cancel()
	o, err := object.New(p, object.WithConfigData(body))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	configurer := o.(object.Configurer)
	alerts, err := configurer.ValidateConfig(ctx.Request().Context())
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	if alerts.HasError() {
		// Say which keywords are wrong: the error of the validation is nil
		// when it ran and found them.
		return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s", alerts.Errors().StringWithoutMeta())
	}
	if err := refuseClaimOverrun(ctx.Request().Context(), p, o, configurer.Config()); err != nil {
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "%s", err)
	}
	var knownSub *pubsub.Subscription
	if waitKnown {
		knownSub = a.subscribeObjectKnown(fmt.Sprintf("api.write_object_config_file.known %s %s", p, ctx.Get("uuid")), p)
		defer func() { _ = knownSub.Stop() }()
	}
	var sub *pubsub.Subscription
	if waiting {
		// Subscribed before the write, so no landing is missed between the
		// write and the wait.
		sub = a.subscribeConfigPropagation(fmt.Sprintf("api.write_object_config_file %s %s", p, ctx.Get("uuid")), p)
		defer func() { _ = sub.Stop() }()
	}
	// Use the non-validating commit func as we already validate to emit an explicit error
	if err := base.commit(configurer.Config(), configurer.Config().RecommitInvalid); errors.Is(err, ErrConfigChanged) {
		return JSONProblemf(ctx, http.StatusConflict, "Commit", "%s", err)
	} else if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
	}
	a.announceConfigFileWritten(p)
	if waitKnown {
		knownCtx, knownCancel := context.WithTimeout(ctx.Request().Context(), objectKnownTimeout)
		known := a.waitObjectKnown(knownCtx, knownSub, p)
		knownCancel()
		if !known {
			return JSONProblemf(ctx, http.StatusRequestTimeout, "Object not known yet",
				"%s is created, and not known to this daemon after %s", p, objectKnownTimeout)
		}
	}
	warnSharedRootlessAccounts(ctx, p)
	// Answer with the timestamp the configuration now carries, so the caller
	// can require it of the actions it goes on to ask of the instances. This
	// write lands on the peer nodes a moment after it is acknowledged here,
	// and without the timestamp a caller has nothing to name the configuration
	// it just wrote.
	mtime := file.ModTime(p.ConfigFile())
	if !mtime.IsZero() {
		ctx.Response().Header().Add(api.HeaderLastModified, mtime.Format(time.RFC3339Nano))
	}
	// Asked to wait, the answer is held until the configuration has reached
	// every live node of its scope. The write is done either way: a wait that
	// expires says where the configuration has not landed yet, and is not a
	// failure of the write.
	if waiting && !mtime.IsZero() {
		scope := configScope(p, configurer)
		lagging := a.waitConfigPropagatedTo(waitCtx, sub, func() []string {
			return a.configLaggardsIn(p, mtime, scope)
		})
		if len(lagging) > 0 {
			return JSONProblemf(ctx, http.StatusRequestTimeout, "Configuration still propagating",
				"the configuration of %s is committed as of %s, and has not reached %s before the wait expired",
				p, mtime.Format(time.RFC3339Nano), strings.Join(lagging, ", "))
		}
	}
	return ctx.NoContent(http.StatusNoContent)
}

// configScope is the nodes a configuration is installed on, as the instance
// config manager reads them: the cluster nodes for the cluster configuration,
// the nodes keyword for any other.
func configScope(p naming.Path, configurer object.Configurer) []string {
	if p.Kind == naming.KindCcfg {
		return clusternode.Get()
	}
	v, err := configurer.Config().Eval(key.New("DEFAULT", "nodes"))
	if err != nil {
		return nil
	}
	l, _ := v.([]string)
	return l
}

func (a *DaemonAPI) writeNodeConfigFile(ctx echo.Context, nodename string) error {
	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read body", "%s", err)
	}
	o, err := object.NewNode(object.WithConfigData(body))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	alerts, err := o.ValidateConfig()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	if alerts.HasError() {
		return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s", err)
	}
	// Use the non-validating commit func as we already validate to emit an explicit error
	if err := o.Config().RecommitInvalid(); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
