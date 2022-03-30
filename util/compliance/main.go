package compliance

import (
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/collector"
	"opensvc.com/opensvc/core/rawconfig"
)

type (
	T struct {
		collectorClient *collector.Client
		log             zerolog.Logger
		varDir          string

		// variable
		rulesets Rulesets
	}
)

func New() *T {
	t := &T{
		log:    log.With().Str("c", "compliance").Logger(),
		varDir: filepath.Join(rawconfig.Node.Paths.Var, "compliance"),
	}
	return t
}

func (t *T) SetLogger(v zerolog.Logger) {
	t.log = v.With().Str("c", "compliance").Logger()
}

func (t *T) SetCollectorClient(c *collector.Client) {
	t.collectorClient = c
}

func (t *T) SetVarDir(s string) {
	t.varDir = s
}
