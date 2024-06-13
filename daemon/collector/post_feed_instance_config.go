package collector

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/goccy/go-json"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/msgbus"
)

type (
	instanceConfigPost struct {
		Path string `json:"path"`

		Orchestrate string `json:"orchestrate"`

		Topology string `json:"topology"`

		FlexMin    int `json:"flex_min"`
		FlexMax    int `json:"flex_max"`
		FlexTarget int `json:"flex_target"`

		App string `json:"app"`

		Env string `json:"env"`

		Scope    []string `json:"scope"`
		DrpNode  string   `json:"drp_node"`
		DrpNodes []string `json:"drp_nodes"`

		Comment string `json:"comment"`

		RawConfig []byte `json:"raw_config"`
	}
)

func (t *T) sendInstancesConfigChange() (err error) {
	t.log.Debugf("sendInstancesConfigChange")
	for p, v := range t.instanceConfigChange {
		b, err := t.asPostFeedInstanceConfigBody(p, v)
		if err != nil {
			// skip os.ErrNotExist, path may be deleted
			if !errors.Is(err, os.ErrNotExist) {
				t.log.Warnf("skip send instance config %s@%s: %s", v.Path, v.Node, err)
			}
			continue
		} else if len(b) == 0 {
			t.log.Warnf("skip send instance config %s@%s: empty body", v.Path, v.Node)
			continue
		}

		if err := t.doPostInstanceConfig(b, v.Path, v.Node); err != nil {
			t.log.Warnf("post instance config %s@%s: %s", v.Path, v.Node, err)
			continue
		}
	}
	t.instanceConfigChange = make(map[naming.Path]*msgbus.InstanceConfigUpdated)
	return
}

func (t *T) asPostFeedInstanceConfigBody(p naming.Path, v *msgbus.InstanceConfigUpdated) ([]byte, error) {
	if v == nil {
		return []byte{}, fmt.Errorf("asPostFeedInstanceConfigBody called with nil InstanceConfigUpdated")
	}
	path := v.Path.String()
	if len(path) == 0 {
		return []byte{}, fmt.Errorf("asPostFeedInstanceConfigBody called with empty path")
	}
	value := v.Value
	pa := instanceConfigPost{
		Path:        v.Path.String(),
		Orchestrate: value.Orchestrate,
		Topology:    value.Topology.String(),
		FlexMin:     value.FlexMin,
		FlexMax:     value.FlexMax,
		FlexTarget:  value.FlexTarget,
		App:         value.App,
		Env:         value.Env,
		Scope:       value.Scope,
	}

	// TODO: set DrpNode, DrpNodes, Comment, encap

	if rawConfig, err := os.ReadFile(p.ConfigFile()); err != nil {
		return []byte{}, err
	} else {
		pa.RawConfig = rawConfig
	}

	return json.Marshal(pa)
}

func (t *T) doPostInstanceConfig(b []byte, p naming.Path, nodename string) error {
	method := http.MethodPost
	path := "/oc3/feed/object/config"
	t.log.Infof("%s %s %s@%s", method, path, p, nodename)
	resp, err := t.client.DoRequest(method, path, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("%s %s: %s", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusAccepted:
		t.log.Debugf("%s %s %s@%s status code %d", method, path, p, nodename, resp.StatusCode)
		return nil
	default:
		return fmt.Errorf("%s %s unexpected status code: %d", method, path, resp.StatusCode)
	}
}
