package daemonapi

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/api"
)

func TestValidateNodeActionWolParams(t *testing.T) {
	ports := func(l ...int) *[]int { return &l }
	t.Run("accepts valid parameters", func(t *testing.T) {
		for name, params := range map[string]api.PostNodeActionWolParams{
			"default port": {Mac: []string{"52:54:00:aa:bb:cc"}, Broadcast: []string{"10.0.0.255"}},
			"lists":        {Mac: []string{"52:54:00:aa:bb:cc", "52-54-00-aa-bb-cd"}, Broadcast: []string{"10.0.0.255", "10.0.1.255"}, Port: ports(7, 9)},
		} {
			assert.NoError(t, validateNodeActionWolParams(params), name)
		}
	})
	t.Run("refuses invalid parameters", func(t *testing.T) {
		for name, params := range map[string]api.PostNodeActionWolParams{
			"no mac":            {Broadcast: []string{"10.0.0.255"}},
			"no broadcast":      {Mac: []string{"52:54:00:aa:bb:cc"}},
			"flag-like mac":     {Mac: []string{"--help"}, Broadcast: []string{"10.0.0.255"}},
			"invalid broadcast": {Mac: []string{"52:54:00:aa:bb:cc"}, Broadcast: []string{"ff02::1"}},
			"invalid port":      {Mac: []string{"52:54:00:aa:bb:cc"}, Broadcast: []string{"10.0.0.255"}, Port: ports(0)},
		} {
			assert.Error(t, validateNodeActionWolParams(params), name)
		}
	})
}
