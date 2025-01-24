package daemonapi

import (
	"net/http"

	"github.com/iancoleman/orderedmap"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonAPI) GetNodeConfig(ctx echo.Context, nodename string, params api.GetNodeConfigParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if a.localhost == nodename {
		return a.GetLocalNodeConfig(ctx, nodename, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeConfig(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) GetLocalNodeConfig(ctx echo.Context, nodename string, params api.GetNodeConfigParams) error {
	var (
		evaluate    bool
		impersonate string
	)
	if params.Evaluate != nil {
		evaluate = *params.Evaluate
	}
	if params.Impersonate != nil {
		impersonate = *params.Impersonate
	}
	var err error
	var data *orderedmap.OrderedMap
	logName := "GetNodeConfig"
	log := LogHandler(ctx, logName)
	log.Debugf("%s: starting", logName)

	if impersonate != "" && !evaluate {
		// Force evaluate when impersonate
		evaluate = true
	}
	filename := rawconfig.NodeConfigFile()
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Errorf("configFile no present(mtime) %s %s (may be deleted)", filename, mtime)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "configFile no present(mtime) %s %s (may be deleted)", filename, mtime)
	}

	data, err = nodeConfigData(evaluate, impersonate)
	if err != nil {
		log.Errorf("can't get configData for %s", filename)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error TODO", "can't get configData for %s", filename)
	}
	if file.ModTime(filename) != mtime {
		log.Errorf("file has changed %s", filename)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error TODO", "file has changed %s", filename)
	}
	data.Set("metadata", map[string]string{
		"node": nodename,
	})
	resp := api.ObjectConfig{
		Data:  *data,
		Mtime: mtime,
	}
	return ctx.JSON(http.StatusOK, resp)
}

func nodeConfigData(eval bool, impersonate string) (data *orderedmap.OrderedMap, err error) {
	var config rawconfig.T
	o, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return
	}
	if eval {
		if impersonate != "" {
			config, err = o.EvalConfigAs(impersonate)
		} else {
			config, err = o.EvalConfig()
		}
	} else {
		config, err = o.PrintConfig()
	}
	if err != nil {
		return
	}
	return config.Data, nil
}
