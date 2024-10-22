package palette

import "github.com/fatih/color"

// The color names as string, usable in configuration files.
const (
	DefaultPrimary   = "himagenta"
	DefaultSecondary = "hiblack"
	DefaultOptimal   = "higreen"
	DefaultError     = "hired"
	DefaultWarning   = "hiyellow"
	DefaultFrozen    = "hiblue"
)

type (
	// C is the integer reprenstation of the color (ANSI code).
	C color.Attribute

	// StringPalette declares the color (as string) to use for each role.
	StringPalette struct {
		Primary   string
		Secondary string
		Optimal   string
		Error     string
		Warning   string
		Frozen    string
	}

	// ColorPalette declares the color (as C) to use for each role.
	ColorPalette struct {
		Primary   color.Attribute
		Secondary color.Attribute
		Optimal   color.Attribute
		Error     color.Attribute
		Warning   color.Attribute
		Frozen    color.Attribute
		Bold      color.Attribute
	}

	// ColorPaletteFunc declares the string colorizer to use for each role.
	ColorPaletteFunc struct {
		Primary   func(a ...any) string
		Secondary func(a ...any) string
		Optimal   func(a ...any) string
		Error     func(a ...any) string
		Warning   func(a ...any) string
		Frozen    func(a ...any) string
		Bold      func(a ...any) string
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

// New returns a color palette (as C) from a string color palette (as read by viper).
func New(m StringPalette) *ColorPalette {
	r := &ColorPalette{}
	r.Primary = toFgColor(m.Primary)
	r.Secondary = toFgColor(m.Secondary)
	r.Optimal = toFgColor(m.Optimal)
	r.Error = toFgColor(m.Error)
	r.Warning = toFgColor(m.Warning)
	r.Frozen = toFgColor(m.Frozen)
	r.Bold = color.Bold
	return r
}

// NewFunc returns a color palette (as string colorizer func) from a string color palette (as read by viper).
func NewFunc(m StringPalette) *ColorPaletteFunc {
	r := &ColorPaletteFunc{}
	c := New(m)
	r.Primary = color.New(c.Primary).SprintFunc()
	r.Secondary = color.New(c.Secondary).SprintFunc()
	r.Optimal = color.New(c.Optimal).SprintFunc()
	r.Error = color.New(c.Error).SprintFunc()
	r.Warning = color.New(c.Warning).SprintFunc()
	r.Frozen = color.New(c.Frozen).SprintFunc()
	r.Bold = color.New(c.Bold).SprintFunc()
	return r
}

func DefaultPalette() *StringPalette {
	return &StringPalette{
		Primary:   DefaultPrimary,
		Secondary: DefaultSecondary,
		Optimal:   DefaultOptimal,
		Error:     DefaultError,
		Warning:   DefaultWarning,
		Frozen:    DefaultFrozen,
	}
}

func DefaultColorPalette() *ColorPalette {
	return New(*DefaultPalette())
}

func DefaultFuncPalette() *ColorPaletteFunc {
	return NewFunc(*DefaultPalette())
}
