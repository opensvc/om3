package object

import (
	"fmt"

	"opensvc.com/opensvc/core/keyop"
)

// OptsSet is the options of the Set object method.
type OptsSet struct {
	Global     OptsGlobal
	Lock       OptsLocking
	KeywordOps []string `flag:"kwops"`
}

// Set gets a keyword value
func (t *Base) Set(options OptsSet) error {
	changes := 0
	for _, kw := range options.KeywordOps {
		op := keyop.Parse(kw)
		if op.IsZero() {
			return fmt.Errorf("invalid set expression: %s", kw)
		}
		t.log.Debug().
			Str("key", op.Key.String()).
			Str("op", op.Op.String()).
			Str("val", op.Value).
			Msg("set")
		if err := t.config.Set(*op); err != nil {
			return err
		}
		changes++
	}
	if changes > 0 {
		return t.config.Commit()
	}
	return nil
}
