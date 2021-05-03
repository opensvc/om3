package instance

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInstanceStatusUnmarshalJSON(t *testing.T) {
	var instanceStatus Status
	path := filepath.Join("test-fixtures", "instanceStatus.json")
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err)
	err = json.Unmarshal(b, &instanceStatus)
	require.Nil(t, err)
}
