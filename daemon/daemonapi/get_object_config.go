package daemonapi

import (
	"net/http"

	"github.com/iancoleman/orderedmap"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonAPI) GetObjectConfig(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectConfigParams) error {
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
	logName := "GetObjectConfig"
	log := LogHandler(ctx, logName)
	log.Debugf("%s: starting", logName)

	objPath, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Infof("%s: %s", logName, err)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Parameter", "invalid path: %s", err)
	}
	if impersonate != "" && !evaluate {
		// Force evaluate when impersonate
		evaluate = true
	}

	if instConfig := instance.ConfigData.Get(objPath, a.localhost); instConfig != nil {
		filename := objPath.ConfigFile()
		mtime := file.ModTime(filename)
		if mtime.IsZero() {
			log.Errorf("config file not found: %s", filename)
			return JSONProblemf(ctx, http.StatusNotFound, "Not Found", "config file no found: %s", filename)
		}

		data, err = configData(objPath, evaluate, impersonate)
		if err != nil {
			log.Errorf("can't get configData for %s %s", objPath, filename)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "can't get configData for %s %s", objPath, filename)
		}
		if file.ModTime(filename) != mtime {
			log.Errorf("file has changed: %s", filename)
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "file has changed: %s", filename)
		}
		respData := make(map[string]interface{})
		respData["metadata"] = objPath.ToMetadata()
		for _, k := range data.Keys() {
			if v, ok := data.Get(k); ok {
				respData[k] = v
			}
		}
		data.Set("metadata", objPath.ToMetadata())
		resp := api.ObjectConfig{
			Data:  respData,
			Mtime: mtime,
		}
		return ctx.JSON(http.StatusOK, resp)
	}
	for nodename, _ := range instance.ConfigData.GetByPath(objPath) {
		return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
			return c.GetObjectConfig(ctx.Request().Context(), namespace, kind, name, &params)
		})
	}
	return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Object not found: %s", objPath)

}

func configData(p naming.Path, eval bool, impersonate string) (data *orderedmap.OrderedMap, err error) {
	var o object.Configurer
	var config rawconfig.T
	if o, err = object.NewConfigurer(p, object.WithVolatile(true)); err != nil {
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
