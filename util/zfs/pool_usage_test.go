package zfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A pool of zvols reserving most of it, none written yet: zpool says it free,
// and a zfs create of a gigabyte finds no space.
func TestParsePoolUsage(t *testing.T) {
	u, err := parsePoolUsage([]byte("tank\tused\t3161056768\t-\ntank\tavailable\t731257344\t-\n"))
	require.NoError(t, err)
	require.Equal(t, PoolUsage{Alloc: 3161056768, Free: 731257344}, u)
}

func TestParsePoolUsageRefusesWhatItDoesNotKnow(t *testing.T) {
	_, err := parsePoolUsage([]byte("cannot open 'tank': dataset does not exist\n"))
	require.Error(t, err)
	_, err = parsePoolUsage([]byte("tank\tused\tnone\t-\ntank\tavailable\t1\t-\n"))
	require.Error(t, err)
}
