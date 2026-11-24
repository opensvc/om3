package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

func TestValidateNodeActionWakeParams(t *testing.T) {
	isClusterNode := func(s string) bool { return s == "n1" || s == "n2" }
	macs := func(l ...string) *[]string { return &l }
	t.Run("accepts valid parameters", func(t *testing.T) {
		for name, params := range map[string]api.PostNodeActionWakeParams{
			"configured mac": {Target: "n2"},
			"mac override":   {Target: "n2", Mac: macs("22:23:24:2d:01:12", "22-23-24-2d-02-12")},
		} {
			assert.NoError(t, validateNodeActionWakeParams(params, isClusterNode), name)
		}
	})
	t.Run("refuses invalid parameters", func(t *testing.T) {
		for name, params := range map[string]api.PostNodeActionWakeParams{
			"no target":          {},
			"not a cluster node": {Target: "n9"},
			"flag-like target":   {Target: "--help"},
			"invalid mac":        {Target: "n2", Mac: macs("bad")},
		} {
			assert.Error(t, validateNodeActionWakeParams(params, isClusterNode), name)
		}
	})
}
