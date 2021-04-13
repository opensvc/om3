package output

import (
	"encoding/json"
	"fmt"
	"regexp"

	"opensvc.com/opensvc/config"
	"opensvc.com/opensvc/util/render"
)

type (
	// RenderFunc is the protype of human format renderer functions.
	RenderFunc func() string

	// Renderer hosts the renderer options and data, and exposes the rendering
	// method.
	Renderer struct {
		Format        string
		Color         string
		Data          interface{}
		HumanRenderer RenderFunc
	}
)

var (
	indent              = "    "
	regexpJSONKey       = regexp.MustCompile(`(".+":)`)
	regexpJSONReference = regexp.MustCompile(`({[\w.-_:]+})`)
	regexpJSONScope     = regexp.MustCompile(`(@.+)(":)`)
	regexpJSONErrors    = regexp.MustCompile(`(")(down|stdby down|err|error)(")`)
	regexpJSONOptimal   = regexp.MustCompile(`(")(up|stdby up|ok)(")`)
	regexpJSONWarning   = regexp.MustCompile(`(")(warn)(")`)
	regexpJSONSecondary = regexp.MustCompile(`(")(n/a)(")`)
)

//
// Sprint returns the string representation of the data in one of the
// supported format (json, flat, human, ...).
//
// The human format needs a RenderFunc to be passed.
//
func (t Renderer) Sprint() string {
	format := toID[t.Format]
	render.SetColor(t.Color)
	switch format {
	case Flat:
		b, _ := json.Marshal(t.Data)
		return SprintFlat(b)
	case JSON:
		b, _ := json.MarshalIndent(t.Data, "", indent)
		s := string(b) + "\n"
		s = regexpJSONKey.ReplaceAllString(s, config.Node.Colorize.Primary("$1"))
		s = regexpJSONReference.ReplaceAllString(s, config.Node.Colorize.Optimal("$1"))
		s = regexpJSONScope.ReplaceAllString(s, config.Node.Colorize.Error("$1")+"$2")
		s = regexpJSONErrors.ReplaceAllString(s, "$1"+config.Node.Colorize.Error("$2")+"$3")
		s = regexpJSONOptimal.ReplaceAllString(s, "$1"+config.Node.Colorize.Optimal("$2")+"$3")
		s = regexpJSONWarning.ReplaceAllString(s, "$1"+config.Node.Colorize.Warning("$2")+"$3")
		s = regexpJSONSecondary.ReplaceAllString(s, "$1"+config.Node.Colorize.Secondary("$2")+"$3")
		return s
	case JSONLine:
		b, _ := json.Marshal(t.Data)
		return string(b) + "\n"
	default:
		if t.HumanRenderer != nil {
			return t.HumanRenderer()
		}
		b, _ := json.MarshalIndent(t.Data, "", indent)
		return string(b) + "\n"
	}
}

//
// Print prints the representation of the data in one of the
// supported format (json, flat, human, ...).
//
// The human format needs a RenderFunc to be passed.
//
func (t Renderer) Print() {
	fmt.Print(t.Sprint())
}
