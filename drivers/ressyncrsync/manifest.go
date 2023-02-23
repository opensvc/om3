package ressyncrsync

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/drivers/ressync"
)

var (
	drvID = driver.NewID(driver.GroupSync, "rsync")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	var keywordL []keywords.Keyword
	keywordL = append(keywordL, Keywords...)
	m := manifest.New(drvID, t)
	m.AddContext([]manifest.Context{
		{
			Key:  "path",
			Attr: "Path",
			Ref:  "object.path",
		},
		{
			Key:  "nodes",
			Attr: "Nodes",
			Ref:  "object.nodes",
		},
		{
			Key:  "drpnodes",
			Attr: "DRPNodes",
			Ref:  "object.drpnodes",
		},
		{
			Key:  "objectID",
			Attr: "ObjectID",
			Ref:  "object.id",
		},
	}...)
	m.AddKeyword(keywordL...)
	m.AddKeyword(ressync.BaseKeywords...)
	return m
}
