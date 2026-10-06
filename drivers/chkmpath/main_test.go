package chkmpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const topo = "36001405914d4edee5824f949beacbccc dm-14 LIO-ORG,c29_disk1\n" +
	"size=1.0G features='1 queue_if_no_path' hwhandler='1 alua' wp=rw\n" +
	"`-+- policy='service-time 0' prio=50 status=active\n" +
	"  |- 8:0:0:1  sdq  65:0   active ready running\n" +
	"  `- 6:0:0:1  sdw  65:96  active ready running\n" +
	"mpatha (360050768018107c8f000000000000123) dm-3 IBM,2145\n" +
	"size=10G features='1 queue_if_no_path' hwhandler='0' wp=rw\n" +
	"|-+- policy='service-time 0' prio=50 status=active\n" +
	"| `- 1:0:0:1 sdb 8:16 active ready running\n" +
	"`-+- policy='service-time 0' prio=10 status=enabled\n" +
	"  `- 2:0:0:1 sdc 8:32 failed faulty running\n" +
	": reload: 3600aaaa dm-5 X,Y\n"

func TestParse(t *testing.T) {
	assert.Equal(t, []mpath{
		{wwid: "6001405914d4edee5824f949beacbccc", devs: []string{"/dev/dm-14", "/dev/sdw"}, activePaths: 2},
		{wwid: "60050768018107c8f000000000000123", devs: []string{"/dev/dm-3", "/dev/sdb"}, activePaths: 1},
		{wwid: "3600aaaa", devs: []string{"/dev/dm-5"}, activePaths: 0},
	}, parse([]byte(topo)))
}
