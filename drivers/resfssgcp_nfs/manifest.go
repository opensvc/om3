package resfssgcp_nfs

import (
	"embed"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resfshost"
)

const (
	DefaultPermission = "read-write"
	DefaultProtocol   = "nfs4.1"
	DefaultExclusive  = "false"
	SecretDefaultName = "iam"
)

var (
	//go:embed text
	fs embed.FS

	drvID = driver.NewID(driver.GroupFS, "sgcp_nfs")

	kws = []*keywords.Keyword{
		{
			Attr:     "UUID",
			Option:   "uuid",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/uuid"),
		},
		{
			Attr:     "Host",
			Option:   "host",
			Required: true,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/host"),
		},
		{
			Attr:       "Permission",
			Option:     "permission",
			Candidates: []string{"read-only", "read-write"},
			Default:    DefaultPermission,
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/permission"),
		},
		{
			Attr:      "Exclusive",
			Option:    "exclusive",
			Converter: "bool",
			Default:   DefaultExclusive,
			Scopable:  true,
			Text:      keywords.NewText(fs, "text/kw/exclusive"),
		},
		{
			Attr:       "Protocol",
			Option:     "protocol",
			Candidates: []string{"nfs4.1"},
			Default:    DefaultProtocol, // TODO: move to config
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/protocol"),
		},
		{
			Attr:     "Secret",
			Option:   "secret",
			Default:  SecretDefaultName,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/secret"),
		},
		{
			Attr:     "Endpoint",
			Option:   "endpoint",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/endpoint"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

func (t *T) DriverID() driver.ID {
	return drvID
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.AddKeywords(kws...)
	m.AddKeywords(
		&resfshost.KeywordDevice,
		&resfshost.KeywordMountPoint,
		&resfshost.KeywordMountOptions,
		&resfshost.KeywordStatTimeout,
		&resfshost.KeywordZone,
		&resfshost.KeywordCheckReadEnabled,
	)
	m.AddKeywords(datarecv.Keywords("DataRecv.")...)
	return m
}
