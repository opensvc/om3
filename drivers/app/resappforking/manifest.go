package resappsimple

import (
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/manifest"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/drivers/app"
)

const (
	driverGroup = drivergroup.App
	driverName  = "forking"
)

// T is the driver structure.
type T struct {
	app.T
	Path     path.T   `json:"path"`
	Nodes    []string `json:"nodes"`
	StartCmd string   `json:"start"`
	StopCmd  string   `json:"stop"`
	CheckCmd string   `json:"check"`
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	var keywordL []keywords.Keyword
	keywordL = append(keywordL, app.Keywords...)
	keywordL = append(keywordL, []keywords.Keyword{}...)
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
