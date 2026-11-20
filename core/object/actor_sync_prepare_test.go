package object

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/resourcereqs"
	"github.com/opensvc/om3/v3/util/plog"
)

// preparingResource is a sync resource recording what it is asked, the rest
// of Driver left to the nil interface it embeds.
type preparingResource struct {
	resource.Driver
	rid        string
	will       bool
	optional   bool
	disabled   bool
	prepareErr error
	journal    *[]string
}

func (t *preparingResource) RID() string                          { return t.rid }
func (t *preparingResource) IsDisabled() bool                     { return t.disabled }
func (t *preparingResource) IsActionDisabled() bool               { return false }
func (t *preparingResource) IsOptional() bool                     { return t.optional }
func (t *preparingResource) Log() *plog.Logger                    { return plog.NewDefaultLogger() }
func (t *preparingResource) Requires(string) *resourcereqs.T      { return resourcereqs.New("") }
func (t *preparingResource) WillPrepareSync(context.Context) bool { return t.will }

func (t *preparingResource) PrepareSync(context.Context) (func() error, error) {
	if t.prepareErr != nil {
		*t.journal = append(*t.journal, "fail "+t.rid)
		return nil, t.prepareErr
	}
	*t.journal = append(*t.journal, "snap "+t.rid)
	return func() error {
		*t.journal = append(*t.journal, "release "+t.rid)
		return nil
	}, nil
}

// updateContext is the context of a sync update action.
func updateContext() context.Context {
	return actioncontext.WithProps(context.Background(), actioncontext.SyncUpdate)
}

func quiesceJournal(journal *[]string) func(context.Context) (func(context.Context) error, error) {
	return func(context.Context) (func(context.Context) error, error) {
		*journal = append(*journal, "quiesce")
		return func(context.Context) error {
			*journal = append(*journal, "resume")
			return nil
		}, nil
	}
}

// TestPrepareSyncsTakesTheSnapshotsAtOneMoment pins the v2 grouping: the
// snapshots of the sync resources of the action are all taken before the
// first transfer, under one quiesce resumed right after them, and released
// after the last transfer, the last taken first. A resource taking nothing,
// as a disabled one or one with delay_snap, is not asked.
func TestPrepareSyncsTakesTheSnapshotsAtOneMoment(t *testing.T) {
	var journal []string
	l := []resource.Driver{
		&preparingResource{rid: "sync#1", will: true, journal: &journal},
		&preparingResource{rid: "sync#2", will: false, journal: &journal},
		&preparingResource{rid: "sync#3", will: true, disabled: true, journal: &journal},
		&preparingResource{rid: "sync#4", will: true, journal: &journal},
	}
	release, err := prepareSyncs(updateContext(), plog.NewDefaultLogger(), l, quiesceJournal(&journal))
	require.NoError(t, err)
	journal = append(journal, "transfers")
	release()
	assert.Equal(t, []string{"quiesce", "snap sync#1", "snap sync#4", "resume", "transfers", "release sync#4", "release sync#1"}, journal)
}

// TestPrepareSyncsQuiescesNothingWithNoSnapshot pins that an action whose
// syncs take no snapshot quiesces nothing.
func TestPrepareSyncsQuiescesNothingWithNoSnapshot(t *testing.T) {
	var journal []string
	l := []resource.Driver{&preparingResource{rid: "sync#1", journal: &journal}}
	release, err := prepareSyncs(updateContext(), plog.NewDefaultLogger(), l, quiesceJournal(&journal))
	require.NoError(t, err)
	release()
	assert.Empty(t, journal)
}

// TestPrepareSyncsAbortsOnAFailure pins that a snapshot failing fails the
// action before any transfer, the writes resumed and the snapshots already
// taken released, unless the resource is optional: warned about and left
// out, the others still taken.
func TestPrepareSyncsAbortsOnAFailure(t *testing.T) {
	var journal []string
	l := []resource.Driver{
		&preparingResource{rid: "sync#1", will: true, journal: &journal},
		&preparingResource{rid: "sync#2", will: true, prepareErr: errors.New("no room"), journal: &journal},
		&preparingResource{rid: "sync#3", will: true, journal: &journal},
	}
	_, err := prepareSyncs(updateContext(), plog.NewDefaultLogger(), l, quiesceJournal(&journal))
	assert.ErrorContains(t, err, "sync#2: prepare the sync: no room")
	assert.Equal(t, []string{"quiesce", "snap sync#1", "fail sync#2", "resume", "release sync#1"}, journal)

	journal = nil
	l[1].(*preparingResource).optional = true
	release, err := prepareSyncs(updateContext(), plog.NewDefaultLogger(), l, quiesceJournal(&journal))
	require.NoError(t, err)
	release()
	assert.Equal(t, []string{"quiesce", "snap sync#1", "fail sync#2", "snap sync#3", "resume", "release sync#3", "release sync#1"}, journal)
}

// TestPrepareSyncsTakesNothingWhenTheQuiesceFails pins that a quiesce
// failing takes no snapshot.
func TestPrepareSyncsTakesNothingWhenTheQuiesceFails(t *testing.T) {
	var journal []string
	l := []resource.Driver{&preparingResource{rid: "sync#1", will: true, journal: &journal}}
	quiesce := func(context.Context) (func(context.Context) error, error) {
		return nil, errors.New("app#1: paused already")
	}
	_, err := prepareSyncs(updateContext(), plog.NewDefaultLogger(), l, quiesce)
	assert.ErrorContains(t, err, "quiesce before the snapshots of the syncs: app#1: paused already")
	assert.Empty(t, journal)
}

// TestSyncsLeftOut pins that a run of a part of the snapshot syncs names the
// others, whose snapshots are not of its moment, and that a run taking no
// snapshot names none.
func TestSyncsLeftOut(t *testing.T) {
	var journal []string
	s1 := &preparingResource{rid: "sync#1", will: true, journal: &journal}
	s2 := &preparingResource{rid: "sync#2", will: true, journal: &journal}
	s3 := &preparingResource{rid: "sync#3", will: false, journal: &journal}
	all := []resource.Driver{s1, s2, s3}
	assert.Equal(t, []string{"sync#1"}, syncsLeftOut(updateContext(), []resource.Driver{s2}, all))
	assert.Empty(t, syncsLeftOut(updateContext(), all, all))
	assert.Nil(t, syncsLeftOut(updateContext(), []resource.Driver{s3}, all), "a run taking no snapshot")
}
