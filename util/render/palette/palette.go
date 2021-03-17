package palette

import "github.com/fatih/color"

const (
	DefaultPrimary   = "yellow"
	DefaultSecondary = "hiblack"
	DefaultError     = "red"
	DefaultWarning   = "hiyellow"
)

type (
	C color.Attribute

	StringPalette struct {
		Primary   string
		Secondary string
		Error     string
		Warning   string
	}

	ColorPalette struct {
		Primary   color.Attribute
		Secondary color.Attribute
		Error     color.Attribute
		Warning   color.Attribute
		Bold      color.Attribute
	}
)

func toFgColor(s string) color.Attribute {
	switch s {
	case "black":
		return color.FgBlack
	case "red":
		return color.FgRed
	case "green":
		return color.FgGreen
	case "yellow":
		return color.FgYellow
	case "blue":
		return color.FgBlue
	case "magenta":
		return color.FgMagenta
	case "cyan":
		return color.FgCyan
	case "white":
		return color.FgWhite
	case "hiblack":
		return color.FgHiBlack
	case "hired":
		return color.FgHiRed
	case "higreen":
		return color.FgHiGreen
	case "hiyellow":
		return color.FgHiYellow
	case "hiblue":
		return color.FgHiBlue
	case "himagenta":
		return color.FgHiMagenta
	case "hicyan":
		return color.FgHiCyan
	case "hiwhite":
		return color.FgHiWhite
	default:
		return color.Reset
	}
}

func New(m StringPalette) ColorPalette {
	r := ColorPalette{}
	r.Primary = toFgColor(m.Primary)
	r.Secondary = toFgColor(m.Secondary)
	r.Error = toFgColor(m.Error)
	r.Warning = toFgColor(m.Warning)
	r.Bold = color.Bold
	return r
}
