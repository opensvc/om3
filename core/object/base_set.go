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
	return t.SetKeywords(options.KeywordOps)
}

func (t *Base) SetKeys(kops ...keyop.T) error {
	changes := 0
	for _, op := range kops {
		if op.IsZero() {
			return fmt.Errorf("invalid set expression: %s", op)
		}
		t.log.Debug().
			Stringer("key", op.Key).
			Stringer("op", op.Op).
			Str("val", op.Value).
			Msg("set")
		if err := t.config.Set(op); err != nil {
			return err
		}
		changes++
	}
	if changes > 0 {
		return t.config.Commit()
	}
	return nil
}

func (t *Base) SetKeywords(kws []string) error {
	l := make([]keyop.T, len(kws))
	for i, kw := range kws {
		op := keyop.Parse(kw)
		l[i] = *op
	}
	return t.SetKeys(l...)
}
