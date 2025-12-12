package object

import (
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/util/key"
)

// Eval returns a keyword value
func (t *core) Eval(k key.T) (interface{}, error) {
	return t.EvalAs(k, "")
}

func (t *core) EvalAs(k key.T, impersonate string) (interface{}, error) {
	if actor, ok := t.config.Referrer.(Actor); ok {
		// required to eval references like {<rid>.exposed_devs}
		actor.ConfigureResources()
	}

	v, err := t.config.EvalAs(k, impersonate)
	switch err.(type) {
	case xconfig.ErrPostponedRef:
		// example: disk#1.exposed_devs[0]
		var i interface{} = t
		if actor, ok := i.(Actor); ok {
			actor.ConfigureResources()
			v, err = t.config.EvalAs(k, impersonate)
		}
	}
	return v, err
}
