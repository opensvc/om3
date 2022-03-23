package daemondatatest

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/hbtype"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("test-fixtures", name)
	b, err := ioutil.ReadFile(path)
	require.Nil(t, err)
	return b
}

func LoadFull(t *testing.T, name string) *cluster.NodeStatus {
	t.Helper()
	var full cluster.NodeStatus
	require.Nil(t, json.Unmarshal(loadFixture(t, name), &full))
	return &full
}

func LoadPatch(t *testing.T, name string) *hbtype.Msg {
	t.Helper()
	var msg hbtype.Msg
	require.Nil(t, json.Unmarshal(loadFixture(t, name), &msg))
	return &msg
}
