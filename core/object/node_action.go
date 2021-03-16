package object

import (
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/config"
)

type (
	// NodeAction describes an action to execute on the local node.
	NodeAction struct {
		BaseAction
		Run func() (interface{}, error)
	}
)

// Do finds the action pointed by Action.Method in the node struct and executes it.
func (t *Node) Do(action NodeAction) ActionResult {
	log.Debug().
		Str("action", action.Action).
		Msg("do")
	result := ActionResult{
		Nodename: config.Node.Hostname,
	}
	data, err := action.Run()
	result.Data = data
	result.Error = err
	if result.Error != nil {
		log.Error().
			Str("action", action.Action).
			Err(result.Error).
			Msg("do")
	}
	result.HumanRenderer = func() string {
		return data.(Renderer).Render()
	}
	return result
}
