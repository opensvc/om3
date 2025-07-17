package resexposeenvoy

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
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
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.Add(
		keywords.Keyword{
			Attr:     "ClusterData",
			Option:   "cluster_data",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/cluster_data"),
		},
		keywords.Keyword{
			Attr:     "FilterConfigData",
			Option:   "filter_config_data",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/filter_config_data"),
		},
		keywords.Keyword{
			Attr:      "Port",
			Converter: "int",
			Option:    "port",
			Required:  true,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/port"),
		},
		keywords.Keyword{
			Attr:       "Protocol",
			Candidates: []string{"tcp", "udp"},
			Default:    "tcp",
			Option:     "protocol",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/protocol"),
		},
		keywords.Keyword{
			Attr:        "ListenerAddr",
			DefaultText: keywords.NewText(fs, "text/kw/listener_addr.default"),
			Option:      "listener_addr",
			Scopable:    true,
			Text:        keywords.NewText(fs, "text/kw/listener_addr"),
		},
		keywords.Keyword{
			Attr:        "ListenerPort",
			Converter:   "int",
			DefaultText: keywords.NewText(fs, "text/kw/listener_port.default"),
			Option:      "listener_port",
			Scopable:    true,
			Text:        keywords.NewText(fs, "text/kw/listener_port"),
		},
		keywords.Keyword{
			Attr:      "SNI",
			Converter: "list",
			Option:    "sni",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/sni"),
		},
		keywords.Keyword{
			Attr:       "LBPolicy",
			Candidates: []string{"round robin", "least_request", "ring_hash", "random", "original_dst_lb", "maglev"},
			Default:    "round robin",
			Option:     "lb_policy",
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/lb_policy"),
		},
		keywords.Keyword{
			Attr:     "Gateway",
			Option:   "gateway",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/gateway"),
		},
		keywords.Keyword{
			Attr:      "Vhosts",
			Converter: "list",
			Option:    "vhosts",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/vhosts"),
		},
		keywords.Keyword{
			Attr:      "ListenerCertificates",
			Converter: "list",
			Option:    "listener_certificates",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/listener_certificates"),
		},
		keywords.Keyword{
			Attr:      "ClusterCertificates",
			Converter: "list",
			Option:    "cluster_certificates",
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/cluster_certificates"),
		},
		keywords.Keyword{
			Attr:     "ClusterPrivateKeyFilename",
			Option:   "cluster_private_key_filename",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/cluster_private_key_filename"),
		},
	)
	return m
}
