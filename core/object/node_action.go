package object

import (
	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/util/hostname"
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
	data, err := action.Run()
	result := ActionResult{
		Nodename:      hostname.Hostname(),
		HumanRenderer: func() string { return defaultHumanRenderer(data) },
	}
	result.Data = data
	result.Error = err
	if result.Error != nil {
		log.Error().
			Str("action", action.Action).
			Err(result.Error).
			Msg("do")
	}
	return result
}
