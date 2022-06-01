package daemondata

import "opensvc.com/opensvc/core/path"

type (
	TInstanceId = string
)

func InstanceId(p path.T, node string) TInstanceId {
	return TInstanceId(node + ":" + p.String())
}
