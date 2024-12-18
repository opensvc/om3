package resexposeenvoy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
)

type (
	T struct {
		resource.T
		ClusterData               string   `json:"cluster_data,omitempty"`
		FilterConfigData          string   `json:"filter_config_data,omitempty"`
		Port                      int      `json:"port,omitempty"`
		Protocol                  string   `json:"protocol,omitempty"`
		ListenerAddr              string   `json:"listener_addr,omitempty"`
		ListenerPort              int      `json:"listener_port,omitempty"`
		SNI                       []string `json:"sni,omitempty"`
		LBPolicy                  string   `json:"lb_policy,omitempty"`
		Gateway                   string   `json:"gateway,omitempty"`
		Vhosts                    []string `json:"vhosts,omitempty"`
		ListenerCertificates      []string `json:"listener_certificates,omitempty"`
		ClusterCertificates       []string `json:"cluster_certificates,omitempty"`
		ClusterPrivateKeyFilename string   `json:"cluster_private_key_filename,omitempty"`
	}
)

func New() resource.Driver {
	t := &T{}
	return t
}

func (t T) Start(ctx context.Context) error {
	return nil
}

func (t T) Stop(ctx context.Context) error {
	return nil
}

func (t *T) Status(ctx context.Context) status.T {
	return status.NotApplicable
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t T) Label(_ context.Context) string {
	addr := "0.0.0.0"
	if t.ListenerAddr != "" {
		addr = t.ListenerAddr
	}
	return fmt.Sprintf("%d/%s via %s:%d", t.Port, t.Protocol, addr, t.ListenerPort)
}

func (t T) Provision(ctx context.Context) error {
	return nil
}

func (t T) Unprovision(ctx context.Context) error {
	return nil
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

// StatusInfo implements resource.StatusInfoer
func (t T) StatusInfo(_ context.Context) map[string]interface{} {
	data := make(map[string]interface{})
	cData := make(map[string]interface{})
	fcData := make(map[string]interface{})
	if err := json.Unmarshal([]byte(t.ClusterData), &cData); err == nil {
		data["cluster_data"] = cData
	} else {
		t.StatusLog().Warn("cluster_data kw: %s", err)
	}
	if err := json.Unmarshal([]byte(t.FilterConfigData), &fcData); err == nil {
		data["filter_config_data"] = fcData
	} else {
		t.StatusLog().Warn("filter_config_data kw: %s", err)
	}
	data["port"] = t.Port
	data["protocol"] = t.Protocol
	data["listener_addr"] = t.ListenerAddr
	data["listener_port"] = t.ListenerPort
	data["sni"] = t.SNI
	data["lb_policy"] = t.LBPolicy
	data["gateway"] = t.Gateway
	data["vhosts"] = t.Vhosts
	data["listener_certificates"] = t.ListenerCertificates
	data["cluster_certificates"] = t.ClusterCertificates
	data["cluster_private_key_filename"] = t.ClusterPrivateKeyFilename
	return data
}
