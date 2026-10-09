package btrfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The lines are "btrfs subvolume list -c -u -q -R" of btrfs-progs 6.6, on a
// mount of the root of the filesystem.
const listOutput = `ID 256 gen 12 cgen 7 top level 5 parent_uuid -                                    received_uuid -                                    uuid 690341a6-0d0b-2f42-aa68-b86e0c9e6b2f path data
ID 257 gen 8 cgen 8 top level 5 parent_uuid 690341a6-0d0b-2f42-aa68-b86e0c9e6b2f received_uuid -                                    uuid edbce858-0827-cf48-95f5-d8b8e175c8d5 path .osync/dev/svc/s1/sync#1/20261009T101010.000000Z/_
ID 258 gen 9 cgen 9 top level 5 parent_uuid edbce858-0827-cf48-95f5-d8b8e175c8d5 received_uuid 1cdc980a-8ebb-a846-af6d-9128235fd479 uuid e2f47bb8-23f4-934f-9a1f-49733770eeff path .osync/dev/svc/s1/sync#1/20261009T111010.000000Z/_
ID 259 gen 10 cgen 10 top level 256 parent_uuid - received_uuid - uuid 11111111-0d0b-2f42-aa68-b86e0c9e6b2f path data/with space
`

func TestParseList(t *testing.T) {
	l, err := ParseList([]byte(listOutput))
	require.NoError(t, err)
	require.Len(t, l, 4)

	assert.Equal(t, Subvol{ID: 256, CGen: 7, Path: "data", UUID: "690341a6-0d0b-2f42-aa68-b86e0c9e6b2f"}, l[0])
	assert.Equal(t, "edbce858-0827-cf48-95f5-d8b8e175c8d5", l[1].Identity(), "an own snapshot is its uuid")
	assert.False(t, l[1].IsReceived())
	assert.Equal(t, "1cdc980a-8ebb-a846-af6d-9128235fd479", l[2].Identity(), "a received snapshot is the uuid it was sent from")
	assert.True(t, l[2].IsReceived())
	assert.Equal(t, "edbce858-0827-cf48-95f5-d8b8e175c8d5", l[2].ParentUUID)
	assert.Equal(t, "data/with space", l[3].Path, "the path is the end of the line")
}

func TestParseListRefusesALineItCanNotRead(t *testing.T) {
	_, err := ParseList([]byte("ID 256 gen 12 top level 5\n"))
	assert.Error(t, err, "no path")
	_, err = ParseList([]byte("gen 12 top level 5 path x\n"))
	assert.Error(t, err, "no id")
}

func TestUnder(t *testing.T) {
	l, err := ParseList([]byte(listOutput))
	require.NoError(t, err)
	var paths []string
	for _, s := range Under(l, "/data/") {
		paths = append(paths, s.Path)
	}
	assert.Equal(t, []string{"data", "data/with space"}, paths)
}
