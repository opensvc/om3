package object

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/config"
)

func (t *Node) varDir() string {
	if t.paths.varDir != "" {
		return t.paths.varDir
	}
	p := fmt.Sprintf("%s/%s", config.Node.Paths.Var, "node")
	t.paths.varDir = filepath.FromSlash(p)
	if err := os.MkdirAll(t.paths.varDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.varDir
}

func (t *Node) logDir() string {
	if t.paths.logDir != "" {
		return t.paths.logDir
	}
	p := fmt.Sprintf("%s", config.Node.Paths.Log)
	t.paths.logDir = filepath.FromSlash(p)
	if err := os.MkdirAll(t.paths.logDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.logDir
}

func (t *Node) tmpDir() string {
	if t.paths.tmpDir != "" {
		return t.paths.tmpDir
	}
	p := fmt.Sprintf("%s", config.Node.Paths.Tmp)
	t.paths.tmpDir = filepath.FromSlash(p)
	if err := os.MkdirAll(t.paths.tmpDir, os.ModePerm); err != nil {
		log.Error().Msgf("%s", err)
	}
	return t.paths.tmpDir
}
