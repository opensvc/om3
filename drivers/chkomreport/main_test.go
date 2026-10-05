package chkomreport

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/check"
)

const system = `Health

Main System Chassis

SEVERITY : COMPONENT
Ok       : Main System Chassis

For further help, type the command followed by -?

Main System Chassis

SEVERITY : COMPONENT
Ok       : Fans
Critical : Intrusion
Ok       : Memory
`

func TestResults(t *testing.T) {
	assert.Equal(t, []check.Result{
		{Instance: "main system chassis", Value: 0},
		{Instance: "fans", Value: 0},
		{Instance: "intrusion", Value: 1},
		{Instance: "memory", Value: 0},
	}, results([]byte(system)))
}
