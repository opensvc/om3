package rescontainer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/core/actioncontext"
)

func TestProvisionStarts(t *testing.T) {
	leader := actioncontext.WithLeader(context.Background(), true)
	follower := actioncontext.WithLeader(context.Background(), false)
	assert.True(t, ProvisionStarts(leader, false), "the leader starts the container")
	assert.False(t, ProvisionStarts(follower, false), "another node does not")
	assert.False(t, ProvisionStarts(context.Background(), false), "nor a provision naming no leader")
	assert.True(t, ProvisionStarts(follower, true), "a standby container runs on every node")
}
