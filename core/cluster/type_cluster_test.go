package cluster

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestStatusUnmarshalJSON(t *testing.T) {
	var clusterStatus Status
	path := filepath.Join("test-fixtures", "clusterStatus.json")
	b, err := ioutil.ReadFile(path)
	assert.Nil(t, err)
	err = json.Unmarshal(b, &clusterStatus)
	assert.Nil(t, err)
}
