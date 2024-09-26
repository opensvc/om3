package clusterdump

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusUnmarshalJSON(t *testing.T) {
	var clusterStatus Data
	path := filepath.Join("test-fixtures", "clusterStatus.json")
	b, err := os.ReadFile(path)
	assert.Nil(t, err)
	err = json.Unmarshal(b, &clusterStatus)
	assert.Nil(t, err)
}
