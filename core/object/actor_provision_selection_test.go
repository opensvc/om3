package object

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/actionresdeps"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/testhelper"
)

// conflicting is a resource of the object a provision finds something else
// holding what it would take.
type conflicting struct {
	resource.Driver
}

func (t conflicting) ProvisionConflicts(context.Context) []string {
	return []string{"held by something else"}
}

func newPreflightTestActor(t *testing.T) *actor {
	t.Helper()
	testhelper.Setup(t)
	_, err := SetClusterConfig()
	require.NoError(t, err)
	p, _ := naming.ParsePath("test/svc/svc1")
	o, err := NewSvc(p, WithConfigData([]byte(`
[fs#1]
type = flag

[fs#2]
type = flag

[app#1]
type = forking
start = /bin/true
`)))
	require.NoError(t, err)
	a := &o.actor
	l := make(resource.Drivers, 0)
	for _, r := range a.Resources() {
		if r.RID() == "fs#1" || r.RID() == "fs#2" {
			l = append(l, conflicting{Driver: r})
		} else {
			l = append(l, r)
		}
	}
	a.resources = l
	// A driver, as the ip of a netns its container, registers what its
	// start needs.
	a.actionResourceDeps.Register(actionresdeps.Dep{Action: "start", A: "app#1", B: "fs#2"})
	return a
}

func provisionContext(rid string) context.Context {
	ctx := actioncontext.WithProps(context.Background(), actioncontext.Provision)
	return actioncontext.WithRID(ctx, rid)
}

// The provision preflight checks the resources the action selects, and the
// ones they depend on: "om <obj> volume provision" was refused for a
// filesystem of the object it does not provision.
func TestTheProvisionPreflightChecksTheSelection(t *testing.T) {
	a := newPreflightTestActor(t)

	err := a.lockedProvisionPreflight(provisionContext(""), preflightAll)
	require.ErrorIs(t, err, ErrProvisionConflict)
	assert.ErrorContains(t, err, "fs#1")
	assert.ErrorContains(t, err, "fs#2")

	err = a.lockedProvisionPreflight(provisionContext("fs#1"), preflightAll)
	require.ErrorIs(t, err, ErrProvisionConflict)
	assert.NotContains(t, err.Error(), "fs#2", "fs#2 is not selected")

	err = a.lockedProvisionPreflight(provisionContext("app#1"), preflightAll)
	require.ErrorIs(t, err, ErrProvisionConflict, "fs#2 is a dependency of the provision of app#1")
	assert.NotContains(t, err.Error(), "fs#1")
}

// A provision of a resource the object does not have is refused before its
// preflight, which would check nothing of it, and would refuse the provision
// for what it finds of the other resources selected.
func TestAProvisionOfAMissingResourceIsRefused(t *testing.T) {
	a := newPreflightTestActor(t)
	ctx := actioncontext.WithRID(context.Background(), "fs#1,fs#9")
	err := a.Provision(ctx)
	require.ErrorContains(t, err, "resource does not exist: fs#9")
	assert.NotErrorIs(t, err, ErrProvisionConflict)
}
