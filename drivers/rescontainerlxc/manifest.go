package rescontainerlxc

import (
	"embed"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/rescontainer"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupContainer, "lxc")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc)
	m.AddKeywords(manifest.SCSIPersistentReservationKeywords...)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextObjectID,
		manifest.ContextNodes,
		manifest.ContextDNS,
		keywords.Keyword{
			Aliases:  []string{"container_data_dir"},
			Attr:     "DataDir",
			Example:  "/srv/svc1/data/containers",
			Option:   "data_dir",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/data_dir"),
		},
		keywords.Keyword{
			Attr:         "RootDir",
			Example:      "/srv/svc1/data/containers",
			Option:       "rootfs",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/rootfs"),
		},
		keywords.Keyword{
			Attr:         "ConfigFile",
			Example:      "/srv/svc1/config",
			Option:       "cf",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/cf"),
		},
		keywords.Keyword{
			Attr:         "Template",
			Example:      "ubuntu",
			Option:       "template",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/template"),
		},
		keywords.Keyword{
			Attr:         "TemplateOptions",
			Converter:    "shlex",
			Example:      "--release focal",
			Option:       "template_options",
			Provisioning: true,
			Text:         keywords.NewText(fs, "text/kw/template_options"),
		},
		keywords.Keyword{
			Attr:         "CreateSecretsEnvironment",
			Converter:    "shlex",
			Example:      "CRT=cert1/server.crt PEM=cert1/server.pem",
			Option:       "create_secrets_environment",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_secrets_environment"),
		},
		keywords.Keyword{
			Attr:         "CreateConfigsEnvironment",
			Converter:    "shlex",
			Example:      "CRT=cert1/server.crt PEM=cert1/server.pem",
			Option:       "create_configs_environment",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_configs_environment"),
		},
		keywords.Keyword{
			Attr:         "CreateEnvironment",
			Converter:    "shlex",
			Example:      "FOO=bar BAR=baz",
			Option:       "create_environment",
			Provisioning: true,
			Scopable:     true,
			Text:         keywords.NewText(fs, "text/kw/create_environment"),
		},
		rescontainer.KWRCmd,
		rescontainer.KWName,
		rescontainer.KWHostname,
		rescontainer.KWStartTimeout,
		rescontainer.KWStopTimeout,
		rescontainer.KWPromoteRW,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	return m
}
