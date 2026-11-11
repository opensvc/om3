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

func TestRIDs(t *testing.T) {
	assert.Equal(t, []string{"ip#0", "fs#0", "app#1"}, RIDs("ip#0 fs#0(down,stdby down) app#1(up) ip#0"))
	assert.Nil(t, RIDs(""))
}
