package commoncmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A ccfg command naming no object is for the cluster configuration, the one
// object of its kind, as "om cluster" is. Other kinds select nothing.
func TestMergeSelectorOfACcfgCommandNamingNoObjectIsTheCluster(t *testing.T) {
	assert.Equal(t, "cluster", MergeSelector("", "", "ccfg", ""))
	assert.Equal(t, "cluster", MergeSelector("", "cluster", "ccfg", ""))
	assert.Equal(t, "**", MergeSelector("", "", "ccfg", "**"), "a default of the command wins")
	assert.Equal(t, "", MergeSelector("", "", "svc", ""))
	assert.Equal(t, "", MergeSelector("", "", "", ""))
}
