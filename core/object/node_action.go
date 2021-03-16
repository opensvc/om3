package object

import (
	"github.com/rs/zerolog/log"
)

// Do finds the action pointed by Action.Method in the node struct and executes it.
func (t *Node) Do(action Action) ActionResult {
	log.Debug().
		Str("action", action.Method).
		Msg("do")
	var (
		err    error
		result ActionResult
	)
	switch action.Method {
	case "Checks":
		opts := action.MethodArgs[0].(ActionOptionsNodeChecks)
		data := t.Checks(opts)
		result.Data = data
		result.Error = err
		result.HumanRenderer = func() string {
			return data.Render()
		}
	default:
		log.Error().
			Str("action", action.Method).
			Msg("not implemented")
	}
	return result
}
