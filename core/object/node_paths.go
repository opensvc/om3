package object

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/rawconfig"
)

// NodePaths contains lazy initialized object paths on the node filesystem.
type NodePaths struct {
	varDir    string
	logDir    string
	tmpDir    string
	dnsUDSDir string
}

func (t *Node) VarDir() string {
	if t.paths.varDir != "" {
		return t.paths.varDir
	}
	p := fmt.Sprintf("%s/%s", rawconfig.Node.Paths.Var, "node")
	t.paths.varDir = filepath.FromSlash(p)
	if err := os.MkdirAll(t.paths.varDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.varDir
}

func (t *Node) LogDir() string {
	if t.paths.logDir != "" {
		return t.paths.logDir
	}
	p := fmt.Sprintf("%s", rawconfig.Node.Paths.Log)
	t.paths.logDir = filepath.FromSlash(p)
	if err := os.MkdirAll(t.paths.logDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.logDir
}

func (t *Node) TmpDir() string {
	if t.paths.tmpDir != "" {
		return t.paths.tmpDir
	}
	p := fmt.Sprintf("%s", rawconfig.Node.Paths.Tmp)
	t.paths.tmpDir = filepath.FromSlash(p)
	if err := os.MkdirAll(t.paths.tmpDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.tmpDir
}

func (t *Node) DNSUDSDir() string {
	if t.paths.dnsUDSDir != "" {
		return t.paths.dnsUDSDir
	}
	t.paths.dnsUDSDir = filepath.Join(t.VarDir(), "dns")
	if err := os.MkdirAll(t.paths.dnsUDSDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.dnsUDSDir
}

func (t *Node) DNSUDSFile() string {
	return filepath.Join(t.DNSUDSDir(), "pdns.sock")
}
