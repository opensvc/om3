package objectaction

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/daemon/api"
)

// A refusal the daemon recorded hands its id to the result of the action, a
// conflict answered as a problem keeps its detail only.
func TestARefusedActionKeepsTheIDOfItsRefusal(t *testing.T) {
	id := uuid.New()
	_, err := handleStatusCode(http.StatusConflict, nil, nil, nil, nil, nil, nil,
		&api.OrchestrationRefused{Detail: "no destination node candidate", OrchestrationID: &id}, nil)
	var refused errRefused
	require.True(t, errors.As(err, &refused))
	assert.Equal(t, id, refused.id)
	assert.Equal(t, "error: no destination node candidate", err.Error())

	_, err = handleStatusCode(http.StatusConflict, nil, nil, nil, nil, nil, nil,
		&api.OrchestrationRefused{Detail: "shrink refused"}, nil)
	assert.False(t, errors.As(err, &refused))
	assert.Equal(t, "error: shrink refused", err.Error())
}
