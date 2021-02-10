package env

import (
//	"path/filepath"
)

type AgentPaths struct {
	Root	string
	Bin	string
	Var	string
	Log	string
	Etc	string
	EtcNs	string
	Tmp	string
	Doc	string
	Html	string
	Lock	string
}

var (
	PathRoot = "/opt/opensvc"
	PathBin = "/opt/opensvc/bin"
)

