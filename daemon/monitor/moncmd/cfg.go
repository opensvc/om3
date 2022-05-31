package moncmd

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	CfgFsWatcherCreate struct {
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
		Updated  timestamp.T
		Ctx      context.Context
		Err      chan error
	}
)
