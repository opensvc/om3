package ressharenfs

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/capabilities"
)

// T is the driver structure.
type T struct {
	resource.T
	resource.Restart
	Path      naming.Path `json:"-"`
	SharePath string      `json:"path"`
	ShareOpts string      `json:"opts"`

	// exportPath is the path of the node the path keyword names, which is
	// what is exported.
	exportPath string

	issues              map[string]string
	issuesMissingClient []string
	issuesWrongOpts     []string
	issuesNone          []string
}

var (
	errExportfsNotInstalled = fmt.Errorf("exportfs is not installed (rescan capabilities after install)")
)

func New() resource.Driver {
	return &T{
		issues:              make(map[string]string),
		issuesMissingClient: make([]string, 0),
		issuesWrongOpts:     make([]string, 0),
		issuesNone:          make([]string, 0),
	}
}

// Label implements Label from resource.Driver interface,
// it returns a formatted short description of the Resource
func (t *T) Label(_ context.Context) string {
	return t.SharePath
}

// locate sets the path of the node the path keyword names, in a volume or a
// filesystem of the object, or of the node. The volume or the filesystem is
// required available when checkAvail, for a path to export.
func (t *T) locate(ctx context.Context, checkAvail bool) error {
	resolve := vpath.Locate
	if checkAvail {
		resolve = vpath.Resolve
	}
	target, err := resolve(ctx, t.SharePath, t.Path.Namespace, vpath.ResolverOf(t.GetObject()))
	if err != nil {
		return err
	}
	t.exportPath = target.HostPath
	return nil
}

// Start the Resource
func (t *T) Start(ctx context.Context) error {
	if !capabilities.Has(drvID.Cap()) {
		return errExportfsNotInstalled
	}
	if err := t.locate(ctx, true); err != nil {
		return err
	}
	exported, err := t.isPathExported()
	if err != nil && len(t.issues) == 0 {
		return err
	}
	if exported && len(t.issues) == 0 {
		t.Log().Infof("already up")
		return nil
	}
	return t.start(ctx)
}

// Stop the Resource. The path of a volume is unexported even when the volume
// is no longer available, as it was exported while it was.
func (t *T) Stop(ctx context.Context) error {
	if !capabilities.Has(drvID.Cap()) {
		return errExportfsNotInstalled
	}
	if err := t.locate(ctx, false); err != nil {
		return err
	}
	return t.stop()
}

// Status evaluates and display the Resource status and logs
func (t *T) Status(ctx context.Context) status.T {
	if !capabilities.Has(drvID.Cap()) {
		t.StatusLog().Error("%s", errExportfsNotInstalled)
		return status.NotApplicable
	}
	if err := t.locate(ctx, false); err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	v, err := t.isPathExported()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if !v {
		return status.Down
	}
	return t.statusFromIssues()
}

// statusFromIssues is the status of a path exported: up when exported to
// every client with the options required, warn otherwise, even when no
// client is, as the path is exported all the same.
func (t *T) statusFromIssues() status.T {
	if len(t.issues) == 0 {
		return status.Up
	}
	for _, client := range slices.Sorted(maps.Keys(t.issues)) {
		t.StatusLog().Warn("%s", t.issues[client])
	}
	return status.Warn
}

func (t *T) Provision(ctx context.Context) error {
	return nil
}

func (t *T) Unprovision(ctx context.Context) error {
	return nil
}

func (t *T) Provisioned(ctx context.Context) (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}
