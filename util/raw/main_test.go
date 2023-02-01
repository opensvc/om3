//go:build linux

package raw

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRaw(t *testing.T) {
	log := &zerolog.Logger{}
	ra := New(
		WithLogger(log),
	)
	t.Logf("data")
	if os.Getuid() != 0 {
		t.Skip("skipped for non root user")
	}
	data, err := ra.QueryAll()
	assert.Nil(t, err)
	assert.GreaterOrEqual(t, len(data), 0)
	//
	// BEWARE: uncomment only after setting a proper bdevpath for your env
	//
	//minor, err := ra.Bind("/dev/sda")
	//assert.Nil(t, err)
	//assert.GreaterOrEqual(t, minor, 1)
	//err = ra.Unbind(minor)
	//assert.Nil(t, err)
}
