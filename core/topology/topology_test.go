package topology

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSON(t *testing.T) {
	var topo T
	tests := []struct {
		b []byte
		v T
	}{
		{[]byte("\"failover\""), Failover},
		{[]byte("\"flex\""), Flex},
	}
	for _, test := range tests {
		t.Run(string(test.b), func(t *testing.T) {
			err := json.Unmarshal(test.b, &topo)
			assert.NoError(t, err)
			assert.Equal(t, test.v, topo)
			b2, err := json.Marshal(topo)
			assert.Equal(t, string(test.b), string(b2))
		})
	}
}
