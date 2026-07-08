package om

import (
	"github.com/opensvc/om3/v3/core/commoncmd"
)

var (
	cmdPool       = commoncmd.NewCmdPool()
	cmdPoolVolume = commoncmd.NewCmdPoolVolume()
)

func init() {
	root.AddCommand(
		cmdPool,
	)
	cmdPool.AddCommand(
		cmdPoolVolume,
		newCmdPoolList(),
	)
	cmdPoolVolume.AddCommand(
		newCmdPoolVolumeList(),
	)
}
