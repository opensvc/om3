package main

import (
	"github.com/opensvc/om3/v3/bundle/resfsskel"
	"github.com/opensvc/om3/v3/core/plugins"
)

var Factory = plugins.NewFactory()

func init() {
	Factory.Register(resfsskel.DriverID, resfsskel.New)
}

func main() {}
