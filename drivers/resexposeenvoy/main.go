package resfsdir

import (
	"context"
	"encoding/json"
	"fmt"

	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/converters"
)

const (
	driverGroup = drivergroup.Expose
	driverName  = "envoy"
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

func init() {
	resource.Register(driverGroup, driverName, New)
}

func New() resource.Driver {
	t := &T{}
	return t
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(driverGroup, driverName, t)
	m.AddKeyword([]keywords.Keyword{
		{
			Option:   "cluster_data",
			Attr:     "ClusterData",
			Scopable: true,
			Text:     "The envoy protocol compliant data in json format used to bootstrap the Cluster config messages. Parts of this structure, like endpoints, are amended to reflect the actual cluster state.",
		},
		{
			Option:   "filter_config_data",
			Attr:     "FilterConfigData",
			Scopable: true,
			Text:     "The envoy protocol compliant data in json format used to bootstrap the Listener filter config messages. Parts of this structure, like routes, are amended by more specific keywords.",
		},
		{
			Option:    "port",
			Attr:      "Port",
			Converter: converters.Int,
			Scopable:  true,
			Required:  true,
			Text:      "The port number of the endpoint.",
		},
		{
			Option:     "protocol",
			Attr:       "Protocol",
			Candidates: []string{"tcp", "udp"},
			Default:    "tcp",
			Scopable:   true,
			Text:       "The envoy protocol compliant data in json format used to bootstrap the Listener filter config messages. Parts of this structure, like routes, are amended by more specific keywords.",
		},
		{
			Option:      "listener_addr",
			Attr:        "ListenerAddr",
			Scopable:    true,
			DefaultText: "The main proxy ip address.",
			Text:        "The public ip address to expose from.",
		},
		{
			Option:      "listener_port",
			Attr:        "ListenerPort",
			Converter:   converters.Int,
			Scopable:    true,
			DefaultText: "The expose <port>.",
			Text:        "The public port number to expose from. The special value 0 is interpreted as a request for auto-allocation.",
		},
		{
			Option:    "sni",
			Attr:      "SNI",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The SNI server names to match on the proxy to select this service endpoints. The socket server must support TLS.",
		},
		{
			Option:     "lb_policy",
			Attr:       "LBPolicy",
			Default:    "round robin",
			Scopable:   true,
			Candidates: []string{"round robin", "least_request", "ring_hash", "random", "original_dst_lb", "maglev"},
			Text:       "The name of the envoy cluster load balancing policy.",
		},
		{
			Option:   "gateway",
			Attr:     "Gateway",
			Scopable: true,
			Text:     "The name of the ingress gateway that should handle this expose.",
		},
		{
			Option:    "vhosts",
			Attr:      "Vhosts",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The list of vhost resource identifiers for this expose.",
		},
		{
			Option:    "listener_certificates",
			Attr:      "ListenerCertificates",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The TLS certificates used by the listener.",
		},
		{
			Option:    "cluster_certificates",
			Attr:      "ClusterCertificates",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The TLS certificates used to communicate with cluster endpoints.",
		},
		{
			Option:   "cluster_private_key_filename",
			Attr:     "ClusterPrivateKeyFilename",
			Scopable: true,
			Text:     "Local filesystem data source of the TLS private key used to communicate with cluster endpoints.",
		},
	}...)
	return m
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

func (t T) Label() string {
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

func (t T) StatusInfo() map[string]interface{} {
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
