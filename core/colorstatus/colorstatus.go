package colorstatus

import (
	"github.com/fatih/color"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/util/render/palette"
)

func Sprint(t status.T, colorize *palette.ColorPaletteFunc) string {
	c := color.New(color.Reset).SprintFunc()
	switch t {
	case status.Up:
		c = colorize.Optimal
	case status.Down:
		c = colorize.Error
	case status.Warn:
		c = colorize.Warning
	case status.NotApplicable:
		c = colorize.Secondary
	case status.Undef:
		c = colorize.Secondary
	case status.StandbyUp:
		c = colorize.Optimal
	case status.StandbyDown:
		c = colorize.Error
	case status.StandbyUpWithUp:
		c = colorize.Optimal
	case status.StandbyUpWithDown:
		c = colorize.Optimal
	}
	return c(t.String())
}
