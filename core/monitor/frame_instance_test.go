package monitor

import (
	"testing"
	"time"

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

func TestObjectInstanceRPOBreached(t *testing.T) {
	InitColor()
	at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	defer func() { now = time.Now }()

	now = func() time.Time { return at.Add(-time.Minute) }
	assert.Equal(t, "", sObjectInstanceRPOBreached(instance.Status{RPOBreachedAt: at}), "not yet")
	now = func() time.Time { return at.Add(time.Minute) }
	assert.Equal(t, iconRPOBreached, sObjectInstanceRPOBreached(instance.Status{RPOBreachedAt: at}), "breached since, whatever the status evaluation time")
	assert.Equal(t, "", sObjectInstanceRPOBreached(instance.Status{}), "no copy kept here")
}
