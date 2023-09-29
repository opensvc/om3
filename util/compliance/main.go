package compliance

import (
	"path/filepath"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type (
	T struct {
		collectorClient *collector.Client
		objectPath      naming.Path
		log             zerolog.Logger
		varDir          string

		// variable
		rulesets Rulesets
	}
)

func New() *T {
	t := &T{
		log:    log.With().Str("c", "compliance").Logger(),
		varDir: filepath.Join(rawconfig.Paths.Var, "compliance"),
	}
	return t
}

func (t *T) SetLogger(v zerolog.Logger) {
	t.log = v.With().Str("c", "compliance").Logger()
}

func (t *T) SetCollectorClient(c *collector.Client) {
	t.collectorClient = c
}

func (t *T) SetObjectPath(s naming.Path) {
	t.objectPath = s
}

func (t *T) SetVarDir(s string) {
	t.varDir = s
}
