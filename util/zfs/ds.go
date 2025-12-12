package zfs

import "github.com/opensvc/om3/v3/util/plog"

type (
	Dataset interface {
		GetName() string
		GetLog() *plog.Logger
	}
)
