package resexposeenvoy

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/util/converters"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupExpose, "envoy")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		keywords.Keyword{
			Option:   "cluster_data",
			Attr:     "ClusterData",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/cluster_data"),
		},
		keywords.Keyword{
			Option:   "filter_config_data",
			Attr:     "FilterConfigData",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/filter_config_data"),
		},
		keywords.Keyword{
			Option:    "port",
			Attr:      "Port",
			Converter: converters.Int,
			Scopable:  true,
			Required:  true,
			Text:      keywords.NewText(fs, "text/kw/port"),
		},
		keywords.Keyword{
			Option:     "protocol",
			Attr:       "Protocol",
			Candidates: []string{"tcp", "udp"},
			Default:    "tcp",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/protocol"),
		},
		keywords.Keyword{
			Option:      "listener_addr",
			Attr:        "ListenerAddr",
			Scopable:    true,
			DefaultText: keywords.NewText(fs, "text/kw/listener_addr.default"),
			Text:        keywords.NewText(fs, "text/kw/listener_addr"),
		},
		keywords.Keyword{
			Option:      "listener_port",
			Attr:        "ListenerPort",
			Converter:   converters.Int,
			Scopable:    true,
			DefaultText: keywords.NewText(fs, "text/kw/listener_port.default"),
			Text:        keywords.NewText(fs, "text/kw/listener_port"),
		},
		keywords.Keyword{
			Option:    "sni",
			Attr:      "SNI",
			Converter: converters.List,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/sni"),
		},
		keywords.Keyword{
			Option:     "lb_policy",
			Attr:       "LBPolicy",
			Default:    "round robin",
			Scopable:   true,
			Candidates: []string{"round robin", "least_request", "ring_hash", "random", "original_dst_lb", "maglev"},
			Text:       keywords.NewText(fs, "text/kw/lb_policy"),
		},
		keywords.Keyword{
			Option:   "gateway",
			Attr:     "Gateway",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/gateway"),
		},
		keywords.Keyword{
			Option:    "vhosts",
			Attr:      "Vhosts",
			Converter: converters.List,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/vhosts"),
		},
		keywords.Keyword{
			Option:    "listener_certificates",
			Attr:      "ListenerCertificates",
			Converter: converters.List,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/listener_certificates"),
		},
		keywords.Keyword{
			Option:    "cluster_certificates",
			Attr:      "ClusterCertificates",
			Converter: converters.List,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/cluster_certificates"),
		},
		keywords.Keyword{
			Option:   "cluster_private_key_filename",
			Attr:     "ClusterPrivateKeyFilename",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/cluster_private_key_filename"),
		},
	)
	return m
}
