package daemonapi

import (
	"crypto/md5"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

// PostInstanceResourceInfo is called by the local `om <path> resource info -r`
// after it refreshed the resource info cache. It publishes the
// InstanceResourceInfoUpdated signal the collector speaker subscribes to.
//
// The request has no body: the refreshed cache is read here to checksum it, so
// the signal stays small and the key-values are only fetched by the speaker,
// with GetInstanceResourceInfo, when it reports them.
func (a *DaemonAPI) PostInstanceResourceInfo(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if ok, err := assertStrategy(ctx, daemonauth.StrategyUX); !ok {
		return err
	}
	log := LogHandler(ctx, "PostInstanceResourceInfo")
	nodename = a.parseNodename(nodename)
	if a.localhost != nodename {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "The resource info refresh signal is local only: %s", nodename)
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Warnf("can't make path: %s", err)
		return JSONProblemf(ctx, http.StatusBadRequest, "New path", "%s", err)
	}

	type resInfoCacher interface {
		ResInfoCacheFile() string
	}

	o, err := object.New(p)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New object", "%s", err)
	}
	i, ok := o.(resInfoCacher)
	if !ok {
		return JSONProblemf(ctx, http.StatusBadRequest, "Load info", "Object does not support info: %s", p)
	}

	filename := i.ResInfoCacheFile()
	b, err := os.ReadFile(filename)
	if errors.Is(err, os.ErrNotExist) {
		return JSONProblemf(ctx, http.StatusNotFound, "Read resource info cache", "%s", err)
	} else if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Read resource info cache", "%s", err)
	}
	info, err := os.Stat(filename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Stat resource info cache", "%s", err)
	}

	a.Bus.Pub(&msgbus.InstanceResourceInfoUpdated{
		Path:      p,
		Node:      a.localhost,
		Checksum:  fmt.Sprintf("%x", md5.Sum(b)),
		UpdatedAt: info.ModTime(),
	},
		pubsub.Label{"namespace", p.Namespace},
		pubsub.Label{"path", p.String()},
		a.LabelLocalhost,
		labelOriginAPI,
	)
	return ctx.JSON(http.StatusOK, nil)
}
