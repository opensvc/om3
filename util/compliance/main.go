package compliance

import (
	"path/filepath"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	T struct {
		collectorClient *collector.Client
		objectPath      naming.Path
		log             *plog.Logger
		varDir          string

		// variable
		rulesets Rulesets
	}
)

func New() *T {
	t := &T{
		log:    plog.NewDefaultLogger().WithPrefix("compliance: ").Attr("pkg", "util/compliance"),
		varDir: filepath.Join(rawconfig.Paths.Var, "compliance"),
	}
	return t
}

func (t *T) SetLogger(v *plog.Logger) {
	t.log = v
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
