package resource

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStatusLogUnmarshall(t *testing.T) {
	var statusLogEntry StatusLogEntry
	b := []byte(`"warn:  foo bar "`)
	err := json.Unmarshal(b, &statusLogEntry)
	assert.Nil(t, err)
	assert.Equal(t, StatusLogEntry{"warn", "foo bar"}, statusLogEntry)
}
