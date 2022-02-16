package zfs

import "github.com/rs/zerolog"

type (
	Dataset interface {
		GetName() string
		GetLog() *zerolog.Logger
	}
)
