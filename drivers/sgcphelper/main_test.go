package sgcphelper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/env"
)

func TestUseCache(t *testing.T) {
	const rid = "fs#1"

	// The selections an action can record for the resource: one including
	// it, one not including it. A nil selection records none.
	var (
		selected    = []string{"app#1", rid}
		notSelected = []string{"app#1"}
	)

	cases := []struct {
		name      string
		cache     string
		origin    env.ActionOrigin
		selection []string
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
		// resources it does not touch, unless the cache is disabled.
		{name: "unset user selected", cache: "", origin: env.ActionOriginUser, selection: selected, want: false},
		{name: "unset user not selected", cache: "", origin: env.ActionOriginUser, selection: notSelected, want: true},
		{name: "unset monitor not selected", cache: "", origin: env.ActionOriginDaemonMonitor, selection: notSelected, want: true},
		{name: "unset scheduler selected", cache: "", origin: env.ActionOriginDaemonScheduler, selection: selected, want: true},
		{name: "forced user selected", cache: "1", origin: env.ActionOriginUser, selection: selected, want: true},
		{name: "disabled user not selected", cache: "0", origin: env.ActionOriginUser, selection: notSelected, want: false},

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
			if tc.selection != nil {
				ctx = actioncontext.WithSelectedRIDs(ctx, tc.selection)
			}
			got, err := UseCache(ctx, rid)
			assert.Equal(t, tc.want, got)
			if tc.err {
				assert.ErrorContains(t, err, CacheVar)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
