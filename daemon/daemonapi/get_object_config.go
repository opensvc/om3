package daemonapi

import (
	"encoding/json"
	"net/http"

	"github.com/iancoleman/orderedmap"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/handlers/handlerhelper"
	"github.com/opensvc/om3/util/file"
)

func (a *DaemonApi) GetObjectConfig(w http.ResponseWriter, r *http.Request, params GetObjectConfigParams) {
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
	var b []byte
	var err error
	var data *orderedmap.OrderedMap
	write, log := handlerhelper.GetWriteAndLog(w, r, "GetObjectConfig")
	log.Debug().Msg("starting")

	objPath, err := path.Parse(params.Path)
	if err != nil {
		log.Info().Err(err).Msgf("invalid path: %s", params.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if impersonate != "" && !evaluate {
		// Force evaluate when impersonate
		evaluate = true
	}
	filename := objPath.ConfigFile()
	mtime := file.ModTime(filename)
	if mtime.IsZero() {
		log.Error().Msgf("configFile no present(mtime) %s %s (may be deleted)", filename, mtime)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	data, err = configData(objPath, evaluate, impersonate)
	if err != nil {
		log.Error().Err(err).Msgf("can't get configData for %s %s", objPath, filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if file.ModTime(filename) != mtime {
		log.Error().Msgf("file has changed %s", filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	respData := make(map[string]interface{})
	respData["metadata"] = objPath.ToMetadata()
	for _, k := range data.Keys() {
		if v, ok := data.Get(k); ok {
			respData[k] = v
		}
	}
	data.Set("metadata", objPath.ToMetadata())
	resp := ObjectConfig{
		Data:  &respData,
		Mtime: &mtime,
	}
	b, err = json.Marshal(resp)
	if err != nil {
		log.Error().Err(err).Msgf("marshal response error %s %s", objPath, filename)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := write(b); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func configData(p path.T, eval bool, impersonate string) (data *orderedmap.OrderedMap, err error) {
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
