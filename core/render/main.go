package render

import (
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// SetColor aligns the color package NoColor boolean with command line
// --color flag value and with tty capability
func SetColor(flag string) {
	switch flag {
	case "no":
		color.NoColor = true
	case "yes":
	case "always":
		color.NoColor = false
	default:
		color.NoColor = !isatty.IsTerminal(os.Stdout.Fd())
	}
}
