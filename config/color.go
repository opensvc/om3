package config

import (
	"github.com/fatih/color"
	"opensvc.com/opensvc/core/status"
)

func ColoredStatus(t status.T) string {
	var c color.Attribute
	switch t {
	case status.Up:
		c = Node.Color.Optimal
	case status.Down:
		c = Node.Color.Error
	case status.Warn:
		c = Node.Color.Warning
	case status.NotApplicable:
		c = Node.Color.Secondary
	case status.Undef:
		c = Node.Color.Secondary
	case status.StandbyUp:
		c = Node.Color.Optimal
	case status.StandbyDown:
		c = Node.Color.Error
	case status.StandbyUpWithUp:
		c = Node.Color.Optimal
	case status.StandbyUpWithDown:
		c = Node.Color.Optimal
	default:
		c = color.Reset
	}
	colorize := color.New(c).SprintFunc()
	return colorize(t.String())
}
