package compliance

import (
	"path/filepath"

	"github.com/opensvc/om3/core/collector"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/plog"

	"github.com/rs/zerolog"
)

type (
	T struct {
		collectorClient *collector.Client
		objectPath      naming.Path
		log             plog.Logger
		varDir          string

		// variable
		rulesets Rulesets
	}
)

func New() *T {
	t := &T{
		log:    plog.Logger{
			Logger: plog.GetPkgLogger("compliance"),
			Prefix: "compliance: ",
		},
		varDir: filepath.Join(rawconfig.Paths.Var, "compliance"),
	}
	return t
}

func (t *T) SetLogger(v zerolog.Logger) {
	t.log.Logger = v.With().Str("pkg", "compliance").Logger()
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
