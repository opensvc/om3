package object

import (
	"context"

	"github.com/opensvc/om3/v3/util/key"
)

// Get returns a keyword unevaluated value
func (t *Node) Get(ctx context.Context, kw string) (interface{}, error) {
	k := key.Parse(kw)
	return t.config.Get(k), nil
}

// Eval returns a keyword evaluated value
func (t *Node) Eval(ctx context.Context, kw string) (interface{}, error) {
	k := key.Parse(kw)
	return t.mergedConfig.EvalAs(k, "")
}

// EvalAs returns a keyword evaluated value, as if the evaluator was another node
func (t *Node) EvalAs(ctx context.Context, kw string, impersonate string) (interface{}, error) {
	k := key.Parse(kw)
	return t.mergedConfig.EvalAs(k, impersonate)
}
