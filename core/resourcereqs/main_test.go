package resourcereqs

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/status"
)

func TestResourceRequirements(t *testing.T) {
	definition := "ip#1(up) ip#2(up, stdby up) ip#3"
	t.Logf("requirement definition: %s", definition)
	o := New(definition)
	assert.Equal(t, 3, len(o.Requirements()))
	assert.Equal(t, status.List(status.Up), o.Requirements()["ip#1"])
	assert.Equal(t, status.List(status.Up, status.StandbyUp), o.Requirements()["ip#2"])
	assert.Equal(t, status.List(status.Up, status.StandbyUp), o.Requirements()["ip#3"])
}
