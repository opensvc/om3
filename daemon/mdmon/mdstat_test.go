package mdmon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	mdstatIdle = `Personalities : [raid1] [raid0]
md125 : active raid0 md126[1] md127[0]
      26624 blocks super 1.2 512k chunks

md126 : active raid1 dm-15[1] dm-16[0]
      15360 blocks super 1.2 [2/2] [UU]
      bitmap: 0/1 pages [0KB], 65536KB chunk

md127 : active raid1 dm-18[1] dm-17[0]
      15360 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`
	mdstatResyncing = `Personalities : [raid1] [raid0]
md125 : active raid0 md126[1] md127[0]
      26624 blocks super 1.2 512k chunks

md126 : active raid1 dm-15[1] dm-16[0]
      15360 blocks super 1.2 [2/2] [UU]
      [=>...................]  resync =  8.5% (1312/15360) finish=0.1min speed=1312K/sec
      bitmap: 1/1 pages [4KB], 65536KB chunk

md127 : active raid1 dm-18[1] dm-17[0]
      15360 blocks super 1.2 [2/2] [UU]
      	resync=DELAYED

unused devices: <none>
`
	mdstatResyncFurther = `Personalities : [raid1] [raid0]
md125 : active raid0 md126[1] md127[0]
      26624 blocks super 1.2 512k chunks

md126 : active raid1 dm-15[1] dm-16[0]
      15360 blocks super 1.2 [2/2] [UU]
      [=========>...........]  resync = 48.1% (7392/15360) finish=0.0min speed=1478K/sec
      bitmap: 1/1 pages [4KB], 65536KB chunk

md127 : active raid1 dm-18[1] dm-17[0]
      15360 blocks super 1.2 [2/2] [UU]
      	resync=DELAYED

unused devices: <none>
`
	mdstatFailed = `Personalities : [raid1] [raid0]
md125 : active raid0 md126[1] md127[0]
      26624 blocks super 1.2 512k chunks

md126 : active raid1 dm-15[1](F) dm-16[0]
      15360 blocks super 1.2 [2/1] [U_]
      bitmap: 0/1 pages [0KB], 65536KB chunk

md127 : active raid1 dm-18[1] dm-17[0]
      15360 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`
	mdstatStopped = `Personalities : [raid1] [raid0]
md126 : active raid1 dm-15[1] dm-16[0]
      15360 blocks super 1.2 [2/2] [UU]
      bitmap: 0/1 pages [0KB], 65536KB chunk

md127 : inactive dm-18[1](S) dm-17[0](S)
      30720 blocks super 1.2

md128 : active (auto-read-only) raid1 sdc[1] sdb[0]
      1048000 blocks super 1.2 [2/2] [UU]

unused devices: <none>
`
)

// The progress of an action and the bitmap usage change with no md event:
// they change no signature. The start, the end and the kind of an action do.
func TestTheProgressOfAnActionIsNoChange(t *testing.T) {
	idle := parseMdstat([]byte(mdstatIdle))
	assert.Equal(t, []string{"md125", "md126", "md127"}, changedArrays(nil, idle))
	resyncing := parseMdstat([]byte(mdstatResyncing))
	assert.Equal(t, []string{"md126", "md127"}, changedArrays(idle, resyncing), "a resync started, one delayed")
	assert.Empty(t, changedArrays(resyncing, parseMdstat([]byte(mdstatResyncFurther))), "a resync going on")
	assert.Equal(t, []string{"md126", "md127"}, changedArrays(resyncing, idle), "the resyncs ended")
	assert.Equal(t, "active raid1 dm-15[1] dm-16[0]\n15360 blocks super 1.2 [2/2] [UU]\nresync", parseMdstat([]byte(mdstatResyncing))["md126"])
	assert.Equal(t, "active raid1 dm-18[1] dm-17[0]\n15360 blocks super 1.2 [2/2] [UU]\nresync DELAYED", parseMdstat([]byte(mdstatResyncing))["md127"])
}

// A member failing, an array stopped, started, inactive or read-only is a
// change of that array alone.
func TestTheChangesOfAnArrayNameIt(t *testing.T) {
	idle := parseMdstat([]byte(mdstatIdle))
	assert.Equal(t, []string{"md126"}, changedArrays(idle, parseMdstat([]byte(mdstatFailed))), "a member failed")
	assert.Equal(t, []string{"md125", "md127", "md128"}, changedArrays(idle, parseMdstat([]byte(mdstatStopped))), "stopped, inactive, started")
	assert.Empty(t, changedArrays(idle, parseMdstat([]byte(mdstatIdle))))
}

// A file of no array, or of another format, is no array.
func TestAnEmptyMdstat(t *testing.T) {
	assert.Empty(t, parseMdstat([]byte("Personalities : \nunused devices: <none>\n")))
	assert.Empty(t, parseMdstat(nil))
}
