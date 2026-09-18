package sgcphelper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/env"
)

func TestUseCache(t *testing.T) {
	cases := []struct {
		name   string
		cache  string
		origin env.ActionOrigin
		want   bool
		err    bool
	}{
		{name: "unset user", cache: "", origin: env.ActionOriginUser, want: false},
		{name: "unset scheduler", cache: "", origin: env.ActionOriginDaemonScheduler, want: true},
		{name: "unset monitor", cache: "", origin: env.ActionOriginDaemonMonitor, want: false},
		{name: "unset api", cache: "", origin: env.ActionOriginDaemonAPI, want: false},
		{name: "forced user", cache: "1", origin: env.ActionOriginUser, want: true},
		{name: "forced scheduler", cache: "1", origin: env.ActionOriginDaemonScheduler, want: true},

		{name: "disabled user", cache: "0", origin: env.ActionOriginUser, want: false},
		{name: "disabled scheduler", cache: "0", origin: env.ActionOriginDaemonScheduler, want: false},

		// Only "1" and "0" override the default policy, other values are
		// reported.
		{name: "true user", cache: "true", origin: env.ActionOriginUser, want: false, err: true},
		{name: "yes user", cache: "yes", origin: env.ActionOriginUser, want: false, err: true},
		{name: "false scheduler", cache: "false", origin: env.ActionOriginDaemonScheduler, want: true, err: true},
		{name: "no scheduler", cache: "no", origin: env.ActionOriginDaemonScheduler, want: true, err: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(CacheVar, tc.cache)
			t.Setenv(env.ActionOriginVar, string(tc.origin))
			got, err := UseCache()
			assert.Equal(t, tc.want, got)
			if tc.err {
				assert.ErrorContains(t, err, CacheVar)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
