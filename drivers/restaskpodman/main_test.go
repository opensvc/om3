package restaskpodman

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A task runs its command in a container of the podman container driver, so
// the user it runs rootless as has to reach that container: it is the one
// every podman command of the task is demoted by.
func TestTheTaskContainerIsRunByTheRootlessUser(t *testing.T) {
	task := &T{RootlessUser: "nobody", RootlessGroup: "nogroup"}
	ct := task.container()
	assert.Equal(t, "nobody", ct.RootlessUser)
	assert.Equal(t, "nogroup", ct.RootlessGroup)

	task = &T{}
	assert.Empty(t, task.container().RootlessUser, "a task without rootless_user runs its container as root")
}

// The limit_* keywords of a task are the ulimits of the container of its
// runs, so they have to reach that container.
func TestTheTaskContainerIsGivenTheLimitsOfTheTask(t *testing.T) {
	nofile := int64(1024)
	task := &T{}
	task.Limit.NoFile = &nofile
	ct := task.container()
	if assert.NotNil(t, ct.Limit.NoFile) {
		assert.Equal(t, nofile, *ct.Limit.NoFile)
	}
}
