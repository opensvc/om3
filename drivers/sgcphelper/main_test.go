package sgcphelper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resource"
)

type (
	// fakeObject is the object of fakeResource, only knowing its path.
	fakeObject struct {
		path naming.Path
	}

	// fakeResource implements the part of resource.Driver UseCache reads.
	// Calling any other method panics.
	fakeResource struct {
		resource.Driver
		object any
		rid    string
	}
)

func (t fakeObject) Path() naming.Path { return t.path }

func (t fakeResource) GetObject() any { return t.object }
func (t fakeResource) RID() string    { return t.rid }

func TestUseCache(t *testing.T) {
	const rid = "fs#1"
	var (
		path      = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc1"}
		otherPath = naming.Path{Namespace: "test", Kind: naming.KindSvc, Name: "svc2"}
		r         = fakeResource{object: fakeObject{path: path}, rid: rid}
	)

	// The selections an action can record: one including the resource, one
	// not including it, and one of another object including an rid like
	// this resource's. A zero selection records none.
	type selection struct {
		path naming.Path
		rids []string
	}
	var (
		selected         = selection{path: path, rids: []string{"app#1", rid}}
		notSelected      = selection{path: path, rids: []string{"app#1"}}
		otherNotSelected = selection{path: otherPath, rids: []string{"app#1"}}
		otherSelected    = selection{path: otherPath, rids: []string{"app#1", rid}}
	)

	cases := []struct {
		name      string
		cache     string
		origin    env.ActionOrigin
		selection selection
		want      bool
		err       bool
	}{
		{name: "unset user", cache: "", origin: env.ActionOriginUser, want: false},
		{name: "unset scheduler", cache: "", origin: env.ActionOriginDaemonScheduler, want: true},
		{name: "unset monitor", cache: "", origin: env.ActionOriginDaemonMonitor, want: false},
		{name: "unset api", cache: "", origin: env.ActionOriginDaemonAPI, want: false},
		{name: "forced user", cache: "1", origin: env.ActionOriginUser, want: true},
		{name: "forced scheduler", cache: "1", origin: env.ActionOriginDaemonScheduler, want: true},

		{name: "disabled user", cache: "0", origin: env.ActionOriginUser, want: false},
		{name: "disabled scheduler", cache: "0", origin: env.ActionOriginDaemonScheduler, want: false},

		// An action with a resource selection serves the cache to the
		// resources it does not depend on, unless the cache is disabled.
		{name: "unset user selected", cache: "", origin: env.ActionOriginUser, selection: selected, want: false},
		{name: "unset user not selected", cache: "", origin: env.ActionOriginUser, selection: notSelected, want: true},
		{name: "unset monitor not selected", cache: "", origin: env.ActionOriginDaemonMonitor, selection: notSelected, want: true},
		{name: "unset scheduler selected", cache: "", origin: env.ActionOriginDaemonScheduler, selection: selected, want: true},
		{name: "forced user selected", cache: "1", origin: env.ActionOriginUser, selection: selected, want: true},
		{name: "disabled user not selected", cache: "0", origin: env.ActionOriginUser, selection: notSelected, want: false},

		// The selection of an action on another object, like the one
		// evaluating this object status for its start affinity checks,
		// does not apply to this resource.
		{name: "unset user other object not selected", cache: "", origin: env.ActionOriginUser, selection: otherNotSelected, want: false},
		{name: "unset user other object selected", cache: "", origin: env.ActionOriginUser, selection: otherSelected, want: false},

		// Only "1" and "0" override the default policy, other values are
		// reported.
		{name: "true user", cache: "true", origin: env.ActionOriginUser, want: false, err: true},
		{name: "yes user", cache: "yes", origin: env.ActionOriginUser, want: false, err: true},
		{name: "false scheduler", cache: "false", origin: env.ActionOriginDaemonScheduler, want: true, err: true},
		{name: "no scheduler", cache: "no", origin: env.ActionOriginDaemonScheduler, want: true, err: true},
		{name: "true user not selected", cache: "true", origin: env.ActionOriginUser, selection: notSelected, want: true, err: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(CacheVar, tc.cache)
			t.Setenv(env.ActionOriginVar, string(tc.origin))
			ctx := context.Background()
			if tc.selection.rids != nil {
				ctx = actioncontext.WithSelectedRIDs(ctx, tc.selection.path, tc.selection.rids)
			}
			got, err := UseCache(ctx, r)
			assert.Equal(t, tc.want, got)
			if tc.err {
				assert.ErrorContains(t, err, CacheVar)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	t.Run("resource without object", func(t *testing.T) {
		t.Setenv(CacheVar, "")
		t.Setenv(env.ActionOriginVar, string(env.ActionOriginUser))
		ctx := actioncontext.WithSelectedRIDs(context.Background(), path, notSelected.rids)
		got, err := UseCache(ctx, fakeResource{rid: rid})
		assert.NoError(t, err)
		assert.False(t, got, "a resource with no object can not be matched with the selection")
	})
}
