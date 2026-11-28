package daemonapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNodeImageSave sends the archive of an image of a store of an engine of
// the node, for the prefetch of a peer to load it with no registry.
//
// The images of a store are not told apart by namespace, and the store named
// may be the one of any user of the node: root alone is served.
func (a *DaemonAPI) GetNodeImageSave(ctx echo.Context, nodename string, params api.GetNodeImageSaveParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeImageSave(ctx, params)
	}
	return a.getPeerNodeImageSave(ctx, nodename, params)
}

// getPeerNodeImageSave relays the archive a peer sends, as it comes and for
// as long as it takes: an image is large, and a client timeout would cut it.
func (a *DaemonAPI) getPeerNodeImageSave(ctx echo.Context, nodename string, params api.GetNodeImageSaveParams) error {
	log := LogHandler(ctx, "GetNodeImageSave")
	if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	c, err := a.newProxyClient(ctx, nodename, client.WithTimeout(0))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	resp, err := c.GetNodeImageSave(ctx.Request().Context(), nodename, &params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	}
	defer func() { _ = resp.Body.Close() }()
	w := ctx.Response()
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Warnf("relay the image %s of %s: %s", params.Ref, nodename, err)
		abortResponse()
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeImageSave(ctx echo.Context, params api.GetNodeImageSaveParams) error {
	log := LogHandler(ctx, "GetNodeImageSave")
	pull := resource.Pull{Engine: params.Engine, Image: params.Ref}
	if params.Store != nil {
		pull.Store = *params.Store
	}
	if pull.Image == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'ref' is empty")
	}
	store := resource.ImageStoreOf(pull.Engine)
	if store == nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'engine' with value '%s' is not an engine of the node %s", pull.Engine, a.localhost)
	}
	reqCtx := ctx.Request().Context()

	// The id is sent ahead of the archive, so the store is asked for the
	// image before its save, which is also what tells a missing image
	// while it can still be answered.
	present, id, err := store.Has(reqCtx, pull)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Image store", "%s: %s", pull, err)
	} else if !present {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "%s is not on the node %s", pull, a.localhost)
	}

	// A store not telling the kind of its ids is not refused the save:
	// the requester then compares no id, and looks for the image under
	// its name.
	idKind, err := store.IDKind(reqCtx, pull)
	if err != nil {
		log.Debugf("save %s: %s", pull, err)
	}

	w := ctx.Response()
	w.Header().Set(echo.HeaderContentType, echo.MIMEOctetStream)
	w.Header().Set(api.HeaderImageID, id)
	if idKind != "" {
		w.Header().Set(api.HeaderImageIDKind, idKind)
	}
	w.Header().Set(api.HeaderPlatform, resource.NodePlatform())
	w.WriteHeader(http.StatusOK)

	// A requester of another platform leaves on these headers: they are
	// not held until the engine wrote enough of the archive to fill a
	// buffer.
	w.Flush()

	log.Infof("save %s for %s", pull, userFromContext(ctx).Username)
	savedID, err := store.Save(reqCtx, pull, w)
	switch {
	case errors.Is(err, resource.ErrImageNotFound):
		log.Warnf("save %s: removed from the store since asked for", pull)
		abortResponse()
	case err != nil:
		log.Warnf("save %s: %s", pull, err)
		abortResponse()
	case savedID != id:
		// The reference was given another image between the two
		// questions. The requester compares what it loads with the
		// id it was told, and refuses this archive.
		log.Warnf("save %s: the image saved is %s, and the one announced %s", pull, savedID, id)
	}
	return nil
}

// abortResponse ends a response whose body can not be completed, in the way
// its reader sees as an error: a body merely ended reads as a whole one, and
// an archive cut short would be handed to the load of an engine as if it
// were all of the image. It does not return.
func abortResponse() {
	panic(http.ErrAbortHandler)
}
