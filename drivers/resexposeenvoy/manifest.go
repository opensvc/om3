package resexposeenvoy

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/util/converters"
)

var (
	drvID = driver.NewID(driver.GroupExpose, "envoy")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Add(
		keywords.Keyword{
			Option:   "cluster_data",
			Attr:     "ClusterData",
			Scopable: true,
			Text:     "The envoy protocol compliant data in json format used to bootstrap the Cluster config messages. Parts of this structure, like endpoints, are amended to reflect the actual cluster state.",
		},
		keywords.Keyword{
			Option:   "filter_config_data",
			Attr:     "FilterConfigData",
			Scopable: true,
			Text:     "The envoy protocol compliant data in json format used to bootstrap the Listener filter config messages. Parts of this structure, like routes, are amended by more specific keywords.",
		},
		keywords.Keyword{
			Option:    "port",
			Attr:      "Port",
			Converter: converters.Int,
			Scopable:  true,
			Required:  true,
			Text:      "The port number of the endpoint.",
		},
		keywords.Keyword{
			Option:     "protocol",
			Attr:       "Protocol",
			Candidates: []string{"tcp", "udp"},
			Default:    "tcp",
			Scopable:   true,
			Text:       "The envoy protocol compliant data in json format used to bootstrap the Listener filter config messages. Parts of this structure, like routes, are amended by more specific keywords.",
		},
		keywords.Keyword{
			Option:      "listener_addr",
			Attr:        "ListenerAddr",
			Scopable:    true,
			DefaultText: "The main proxy ip address.",
			Text:        "The public ip address to expose from.",
		},
		keywords.Keyword{
			Option:      "listener_port",
			Attr:        "ListenerPort",
			Converter:   converters.Int,
			Scopable:    true,
			DefaultText: "The expose <port>.",
			Text:        "The public port number to expose from. The special value 0 is interpreted as a request for auto-allocation.",
		},
		keywords.Keyword{
			Option:    "sni",
			Attr:      "SNI",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The SNI server names to match on the proxy to select this service endpoints. The socket server must support TLS.",
		},
		keywords.Keyword{
			Option:     "lb_policy",
			Attr:       "LBPolicy",
			Default:    "round robin",
			Scopable:   true,
			Candidates: []string{"round robin", "least_request", "ring_hash", "random", "original_dst_lb", "maglev"},
			Text:       "The name of the envoy cluster load balancing policy.",
		},
		keywords.Keyword{
			Option:   "gateway",
			Attr:     "Gateway",
			Scopable: true,
			Text:     "The name of the ingress gateway that should handle this expose.",
		},
		keywords.Keyword{
			Option:    "vhosts",
			Attr:      "Vhosts",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The list of vhost resource identifiers for this expose.",
		},
		keywords.Keyword{
			Option:    "listener_certificates",
			Attr:      "ListenerCertificates",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The TLS certificates used by the listener.",
		},
		keywords.Keyword{
			Option:    "cluster_certificates",
			Attr:      "ClusterCertificates",
			Converter: converters.List,
			Scopable:  true,
			Text:      "The TLS certificates used to communicate with cluster endpoints.",
		},
		keywords.Keyword{
			Option:   "cluster_private_key_filename",
			Attr:     "ClusterPrivateKeyFilename",
			Scopable: true,
			Text:     "Local filesystem data source of the TLS private key used to communicate with cluster endpoints.",
		},
	)
	return m
}
