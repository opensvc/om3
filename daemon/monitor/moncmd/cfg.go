package moncmd

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
)

type (
	Exit struct {
		Path     path.T
		Filename string
	}

	CfgFileUpdated struct {
		Path     path.T
		Filename string
	}

	CfgFileRemoved struct {
		Path     path.T
		Filename string
	}

	FrozenFileUpdated struct {
		Path     path.T
		Filename string
	}

	FrozenFileRemoved struct {
		Path     path.T
		Filename string
	}

	CfgDeleted struct {
		Path path.T
		Node string
	}

	CfgUpdated struct {
		Path   path.T
		Node   string
		Config instance.Config
	}

	MonCfgDone struct {
		Path     path.T
		Filename string
	}

	RemoteFileConfig struct {
		Path     path.T
		Node     string
		Filename string
		Updated  time.Time
		Ctx      context.Context
		Err      chan error
	}
)
