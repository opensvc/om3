package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIDWithPrefix(t *testing.T) {
	ids := []string{
		"97a7738c-9dfc-4d00-96d9-2dde44387fc8",
		"97a7738c-9dfc-4d00-96d9-2dde44387fc8",
		"97b00000-0000-4000-8000-000000000000",
		"10000000-0000-4000-8000-000000000000",
	}
	id, err := idWithPrefix("orchestration", "97a7738c", ids)
	require.NoError(t, err, "an id listed twice, by two nodes, is one id")
	assert.Equal(t, "97a7738c-9dfc-4d00-96d9-2dde44387fc8", id)

	_, err = idWithPrefix("orchestration", "97", ids)
	assert.ErrorContains(t, err, "give more of the id")

	_, err = idWithPrefix("orchestration", "ffff", ids)
	assert.ErrorContains(t, err, "no orchestration id the daemons know starts with ffff")
}
