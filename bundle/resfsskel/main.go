package resfsskel

import (
	"context"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/topology"
	"github.com/opensvc/om3/v3/util/capabilities"
)

type T struct {
	resource.T
	Path     naming.Path `json:"path"`
	Nodes    []string    `json:"nodes"`
	Topology topology.T  `json:"topology"`
}

var DriverID = driver.NewID(driver.GroupFS, "skel")

func New() resource.Driver {
	return &T{}
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	return []string{DriverID.Cap()}, nil
}

func init() {
	capabilities.Register(capabilitiesScanner)
}

// Manifest exposes to the core the input expected by the driver.
func (t *T) Manifest() *manifest.T {
	m := manifest.New(DriverID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextTopology,
	)
	return m
}

// Abort is called before a start action. Return true to deny the start.
func (t *T) Abort(ctx context.Context) bool {
	return false
}

func (t *T) Start(ctx context.Context) error {
	t.Log().Infof("noop")
	return nil
}

func (t *T) Stop(ctx context.Context) error {
	t.Log().Infof("noop")
	return nil
}

func (t *T) Label(_ context.Context) string {
	return ""
}

func (t *T) Status(ctx context.Context) status.T {
	t.StatusLog().Info("received path=%s nodes=%s topology=%s", t.Path, t.Nodes, t.Topology)
	return status.NotApplicable
}
