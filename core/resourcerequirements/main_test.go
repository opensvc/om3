package resourcerequirement

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/core/status"
)

func TestResourceRequirements(t *testing.T) {
	definition := "ip#1(up) ip#2(up, stdby up) ip#3"
	t.Logf("requirement definition: %s", definition)
	o := New(definition)
	assert.Equal(t, 3, len(o.Requirements()))
	assert.Equal(t, []status.T{status.Up}, o.Requirements()["ip#1"])
	assert.Equal(t, []status.T{status.Up, status.StandbyUp}, o.Requirements()["ip#2"])
	assert.Equal(t, []status.T{status.Up, status.StandbyUp}, o.Requirements()["ip#3"])
}
