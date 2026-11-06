package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The listings the commands render in the machine formats are the lists the
// api answers with: the renderer sorts their items, by default or as asked,
// only when it can reach them.
func TestListsHandTheirItems(t *testing.T) {
	type getItemser interface {
		GetItems() any
	}
	for _, l := range []any{
		ClusterLockList{}, ExecList{}, OrchestrationList{}, ArrayList{}, StoreList{},
		PoolList{}, PoolVolumeList{}, NetworkList{}, RelayStatusList{},
	} {
		_, ok := l.(getItemser)
		assert.True(t, ok, "%T", l)
	}
}
