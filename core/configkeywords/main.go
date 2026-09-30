// Package configkeywords answers the keywords of an object or node
// configuration, raw or evaluated, as a list of keyword items.
//
// The daemon api answers them, and om answers them the same way when it
// reads the configuration itself, with --local or with the daemon down: the
// two answers are the same list, rendered by the same code.
package configkeywords

import (
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// Options says which keywords to answer, and how.
	Options struct {
		// Keywords are the keywords to answer, and none is the whole
		// configuration.
		Keywords []string

		// Evaluate answers the evaluated value of each keyword beside its
		// raw value.
		Evaluate bool

		// Impersonate is the node the values are evaluated as, the local
		// node when empty.
		Impersonate string

		// Redact hides the value of the secret keywords, for a reader not
		// allowed to see them.
		Redact bool
	}
)

// evaluatedAs is the node the values are evaluated as.
func (o Options) evaluatedAs() string {
	if o.Impersonate != "" {
		return o.Impersonate
	}
	return hostname.Hostname()
}

// keys are the keys to answer: the selection, or every key of the
// configuration.
func (o Options) keys(all key.L) key.L {
	if len(o.Keywords) == 0 {
		return all
	}
	keys := make(key.L, 0, len(o.Keywords))
	for _, s := range o.Keywords {
		keys = append(keys, key.Parse(s))
	}
	return keys
}

// Object answers the keywords of the configuration of an object.
//
// A keyword selection is a user input, so a key it can not evaluate is an
// error, which is xconfig.ErrNoKeyword for a keyword that does not exist.
// The whole configuration is not: it can hold keys no keyword declares, and
// failing on the first one would hide all the others, so each item says
// what failed.
func Object(p naming.Path, o Options) (api.KeywordItems, error) {
	oc, err := object.NewCore(p)
	if err != nil {
		return nil, err
	}
	conf := oc.Config()
	isWholeConfig := len(o.Keywords) == 0
	evaluatedAs := o.evaluatedAs()
	items := make(api.KeywordItems, 0)
	for _, k := range o.keys(conf.KeyList()) {
		item := api.KeywordItem{
			Object:  p.String(),
			Keyword: k.String(),
		}
		if s, err := conf.GetStrict(k); err == nil {
			item.Value = s
		}

		// A secret is not shown, raw or evaluated, to a reader not allowed
		// to see it. The keyword is listed all the same, so the reader
		// knows it is set.
		if o.Redact && object.IsSecretKey(p.Kind, k, conf.GetString(key.New(k.Section, "type"))) {
			item.Value = object.RedactedValue
			if o.Evaluate {
				var v any = object.RedactedValue
				text := object.RedactedValue
				item.Evaluated = &v
				item.EvaluatedText = &text
				item.EvaluatedAs = evaluatedAs
			}
			items = append(items, item)
			continue
		}

		if o.Evaluate {
			v, err := oc.EvalAs(k, evaluatedAs)
			switch {
			case err != nil && isWholeConfig:
				s := err.Error()
				item.Error = &s
				item.EvaluatedAs = evaluatedAs
			case err != nil:
				return nil, err
			default:
				text := conf.EvaluatedText(k, v)
				item.Evaluated = &v
				item.EvaluatedText = &text
				item.EvaluatedAs = evaluatedAs
			}
		}
		items = append(items, item)
	}
	return items, nil
}

// Node answers the keywords of the configuration of the local node, named
// nodename in the items.
//
// The keys of the whole configuration are the ones the node configuration
// sets. The value of a keyword is the one that applies to the node, from its
// configuration or the cluster's, and a keyword set nowhere still evaluates,
// to its default.
func Node(nodename string, o Options) (api.KeywordItems, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	merged := n.MergedConfig()
	isWholeConfig := len(o.Keywords) == 0
	evaluatedAs := o.evaluatedAs()
	items := make(api.KeywordItems, 0)
	for _, k := range o.keys(n.Config().KeyList()) {
		item := api.KeywordItem{
			Node:    nodename,
			Keyword: k.String(),
		}
		if s, err := merged.GetStrict(k); err == nil {
			item.Value = s
		}
		if o.Evaluate {
			v, err := merged.EvalAs(k, evaluatedAs)
			switch {
			case err != nil && isWholeConfig:
				s := err.Error()
				item.Error = &s
				item.EvaluatedAs = evaluatedAs
			case err != nil:
				return nil, err
			default:
				text := merged.EvaluatedText(k, v)
				item.Evaluated = &v
				item.EvaluatedText = &text
				item.EvaluatedAs = evaluatedAs
			}
		}
		items = append(items, item)
	}
	return items, nil
}
