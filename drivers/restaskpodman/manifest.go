package restaskpodman

import (
	"embed"
	"os/exec"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/rescontainer"
	"github.com/opensvc/om3/drivers/restask"
	"github.com/opensvc/om3/drivers/restaskocibase"
)

var (
	//go:embed text
	fs embed.FS
)

var (
	drvID    = driver.NewID(driver.GroupTask, "podman")
	altDrvID = driver.NewID(driver.GroupTask, "oci")
)

func init() {
	driver.Register(drvID, New)
	if _, err := exec.LookPath("docker"); err != nil {
		driver.Register(altDrvID, New)
	}
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
		manifest.ContextObjectID,
		manifest.ContextDNS,
		rescontainer.KWOsvcRootPath,
		rescontainer.KWGuestOS,
	)
	m.AddKeywords(restask.Keywords...)
	m.AddKeywords(restaskocibase.Keywords...)
	m.Add(
		keywords.Keyword{
			Option:   "userns",
			Attr:     "UserNS",
			Scopable: true,
			Example:  "container#0",
			Text:     keywords.NewText(fs, "text/kw/userns"),
		},
	)
	return m
}
