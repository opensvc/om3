package object

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

// A stop, shutdown or unprovision of the whole instance removes the process
// groups of the object, as the v2 agent did, and a scoped one does not: the
// resources it does not stop still run in them.
func TestRemovesPG(t *testing.T) {
	ctx := context.Background()
	for _, props := range []actioncontext.Properties{
		actioncontext.Stop,
		actioncontext.Shutdown,
		actioncontext.Unprovision,
		actioncontext.ProvisionStop,
	} {
		actionCtx := actioncontext.WithProps(ctx, props)
		assert.Truef(t, removesPG(actionCtx, true), "%s", props.Name)
		assert.Falsef(t, removesPG(actionCtx, false), "%s for the encap nodes only", props.Name)
		assert.Falsef(t, removesPG(actioncontext.WithRID(actionCtx, "app#1"), true), "%s --rid", props.Name)
		assert.Falsef(t, removesPG(actioncontext.WithSubset(actionCtx, "g1"), true), "%s --subset", props.Name)
		assert.Falsef(t, removesPG(actioncontext.WithTag(actionCtx, "t1"), true), "%s --tag", props.Name)
		assert.Falsef(t, removesPG(actioncontext.WithTo(actionCtx, "fs#1"), true), "%s --to", props.Name)
	}
	for _, props := range []actioncontext.Properties{
		actioncontext.Start,
		actioncontext.Provision,
		actioncontext.PRStop,
		actioncontext.RestoreStop,
		actioncontext.Delete,
		actioncontext.PGReset,
		actioncontext.PGUpdate,
		actioncontext.Boot,
	} {
		assert.Falsef(t, removesPG(actioncontext.WithProps(ctx, props), true), "%s", props.Name)
	}
}

// The stop half of a restart keeps the process groups of the object, which
// its start half makes again: no group is removed and no slice stopped, for
// the start not to race the stop of the slices on the cgroup v1 hierarchy.
// It is a stop in everything else, and the stop asked for itself still
// removes them.
func TestARestartRemovesNoPG(t *testing.T) {
	props := stopForRestartProps()
	assert.Equal(t, actioncontext.Stop.Name, props.Name)
	assert.False(t, props.RemovesPG)
	assert.False(t, removesPG(actioncontext.WithProps(context.Background(), props), true))
	assert.True(t, actioncontext.Stop.RemovesPG, "the stop is not changed by the restart")
}
