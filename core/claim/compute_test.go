package claim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCPUReadsThePGCPUQuotaNotation(t *testing.T) {
	for s, want := range map[string]int64{
		"50%":      500,
		"100%@4":   4000,
		"10%@2":    200,
		"800%":     8000,
		"100%@all": 16000,
	} {
		v, err := ParseCPU(s, 16)
		require.NoError(t, err, s)
		assert.Equal(t, want, v, s)
	}
}

// A claim limit is a quantity of the cluster, where @all names nothing.
func TestParseCPURefusesAllWithoutANode(t *testing.T) {
	_, err := ParseCPU("100%@all", 0)
	assert.Error(t, err)
	_, err = ParseCPU("half", 4)
	assert.Error(t, err)
}

func TestFormatSaysTheUnit(t *testing.T) {
	assert.Equal(t, "1.5 cpu", Format(TypeCPU, 1500))
	assert.Equal(t, "unbounded", Format(TypeMemory, Unbounded))
}
