package object

import (
	"strings"

	"gopkg.in/errgo.v2/fmt/errors"
	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/key"
)

// OptsSet is the options of the Set object method.
type OptsSet struct {
	Global     OptsGlobal
	Lock       OptsLocking
	KeywordOps []string `flag:"kws"`
}

// Set gets a keyword value
func (t *Base) Set(options OptsSet) error {
	changes := 0
	for _, kw := range options.KeywordOps {
		l := strings.SplitN(kw, "=", 2)
		if len(l) != 2 {
			return errors.Newf("no operator in %s", kw)
		}
		keyPath := l[0]
		val := l[1]
		c := keyPath[len(keyPath)-1]
		var op config.Op
		switch c {
		case '-':
			op = config.OpRemove
			keyPath = keyPath[:len(keyPath)-1]
		case '+':
			op = config.OpAppend
			keyPath = keyPath[:len(keyPath)-1]
		case '|':
			op = config.OpMerge
			keyPath = keyPath[:len(keyPath)-1]
		default:
			op = config.OpSet
		}
		k := key.Parse(keyPath)
		t.log.Debug().
			Str("key", k.String()).
			Int("op", int(op)).
			Str("val", val).
			Msg("set")
		if err := t.config.Set(k, op, val); err != nil {
			return err
		}
		changes++
	}
	if changes > 0 {
		t.config.Commit()
	}
	return nil
}
