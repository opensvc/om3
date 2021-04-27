package object

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func TestInstanceStatusUnmarshalJSON(t *testing.T) {
	var instanceStatus InstanceStatus
	path := filepath.Join("test-fixtures", "instanceStatus.json")
	b, err := ioutil.ReadFile(path)
	assert.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	assert.Nil(t, err)
}
