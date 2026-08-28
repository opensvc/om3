package monitor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resource"
)

func TestObjectInstanceRunning(t *testing.T) {
	InitColor()
	running := resource.RunningInfoList{{RID: "task#3", PID: 2265402}}

	for name, test := range map[string]struct {
		status   instance.Status
		expected string
	}{
		"nothing running": {
			status: instance.Status{},
		},
		"a resource of the instance is running": {
			status:   instance.Status{Running: running},
			expected: iconRunning,
		},
		"a resource of an encapsulated instance is running": {
			status: instance.Status{
				Encap: instance.EncapMap{
					"container#1": instance.EncapStatus{
						Status: instance.Status{Running: running},
					},
				},
			},
			expected: iconRunning,
		},
		"no resource of the encapsulated instance is running": {
			status: instance.Status{
				Encap: instance.EncapMap{
					"container#1": instance.EncapStatus{},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expected, sObjectInstanceRunning(test.status))
		})
	}
}
