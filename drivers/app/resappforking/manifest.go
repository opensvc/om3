package resappforking

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/drivers/app/resappbase"
	"opensvc.com/opensvc/drivers/app/resappunix"
)

const (
	driverGroup = drivergroup.App
	driverName  = "forking"
)

// Manifest ...
func (t T) Manifest() *manifest.T {
	var keywordL []keywords.Keyword
	keywordL = append(keywordL, resappbase.Keywords...)
	keywordL = append(keywordL, resappunix.Keywords...)
	m := manifest.New(driverGroup, driverName)
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
	}...)
	m.AddKeyword(keywordL...)
	return m
}
