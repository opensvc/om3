package compliance

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/collector"
)

type (
	T struct {
		collectorClient *collector.Client
		log             zerolog.Logger

		// variable
		rulesets Rulesets
	}
)

func New() *T {
	t := &T{
		log: log.With().Str("c", "compliance").Logger(),
	}
	return t
}

func (t *T) SetLogger(v zerolog.Logger) {
	t.log = v.With().Str("c", "compliance").Logger()
}

func (t *T) SetCollectorClient(c *collector.Client) {
	t.collectorClient = c
}
