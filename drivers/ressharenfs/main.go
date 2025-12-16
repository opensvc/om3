package ressharenfs

import (
	"context"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/capabilities"
)

// T is the driver structure.
type T struct {
	resource.T
	SharePath string `json:"path"`
	ShareOpts string `json:"opts"`

	issues              map[string]string
	issuesMissingClient []string
	issuesWrongOpts     []string
	issuesNone          []string
}

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

// Start the Resource
func (t *T) Start(ctx context.Context) error {
	if !capabilities.Has("node.x.exportfs") {
		return fmt.Errorf("exportfs is not installed")
	}
	if _, err := t.isPathExported(); err != nil && len(t.issues) == 0 {
		return err
	}
	if t.status() == status.Up {
		t.Log().Infof("already up")
		return nil
	}
	if err := t.start(ctx); err != nil {
		return err
	}
	return nil
}

// Stop the Resource
func (t *T) Stop(ctx context.Context) error {
	if !capabilities.Has("node.x.exportfs") {
		return fmt.Errorf("exportfs is not installed")
	}
	if _, err := t.isPathExported(); err != nil {
		return err
	}
	if t.status() == status.Down {
		t.Log().Infof("already down")
		return nil
	}
	if err := t.stop(); err != nil {
		return err
	}
	return nil
}

// Status evaluates and display the Resource status and logs
func (t *T) Status(ctx context.Context) status.T {
	return t.status()
}

func (t *T) status() status.T {
	if !capabilities.Has("node.x.exportfs") {
		t.StatusLog().Error("exportfs is not installed")
		return status.NotApplicable
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

func (t *T) statusFromIssues() status.T {
	switch len(t.issues) {
	case 0:
		return status.Up
	case len(strings.Fields(t.ShareOpts)):
		return status.Down
	default:
		for _, issue := range t.issues {
			t.StatusLog().Warn("%s", issue)
		}
		return status.Warn
	}
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
