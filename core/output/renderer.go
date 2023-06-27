package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/andreazorzetto/yh/highlight"
	"github.com/fatih/color"
	"github.com/opensvc/om3/util/render"
	"github.com/opensvc/om3/util/render/palette"
	"gopkg.in/yaml.v3"
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
		Colorize      *palette.ColorPaletteFunc
		Stream        bool
	}

	renderer interface {
		Render() string
	}
)

var (
	indent              = "    "
	regexpJSONKey       = regexp.MustCompile(`(".+":)`)
	regexpJSONReference = regexp.MustCompile(`({[\w.#-_:]+})`)
	regexpJSONScope     = regexp.MustCompile(`(@.+)(":)`)
	regexpJSONErrors    = regexp.MustCompile(`(")(down|stdby down|err|error)(")`)
	regexpJSONOptimal   = regexp.MustCompile(`(")(up|stdby up|ok)(")`)
	regexpJSONWarning   = regexp.MustCompile(`(")(warn)(")`)
	regexpJSONSecondary = regexp.MustCompile(`(")(n/a)(")`)
)

// Sprint returns the string representation of the data in one of the
// supported format (json, flat, human, ...).
//
// The human format needs a RenderFunc to be passed.
func (t Renderer) Sprint() string {
	format := toID[t.Format]
	render.SetColor(t.Color)
	if t.Colorize == nil {
		t.Colorize = palette.DefaultFuncPalette()
	}
	switch data := t.Data.(type) {
	case []string:
		if data == nil {
			// JSON Marshal renders "null" for unallocated empty slices
			t.Data = make([]string, 0)
		}
	}
	switch format {
	case Flat:
		b, err := json.Marshal(t.Data)
		if err != nil {
			panic(err)
		}
		if color.NoColor {
			return SprintFlat(b)
		} else {
			return SprintFlatColor(b, t.Colorize)
		}
	case JSON:
		b, err := json.MarshalIndent(t.Data, "", indent)
		if err != nil {
			panic(err)
		}
		s := string(b) + "\n"
		s = regexpJSONKey.ReplaceAllString(s, t.Colorize.Primary("$1"))
		s = regexpJSONReference.ReplaceAllString(s, t.Colorize.Optimal("$1"))
		s = regexpJSONScope.ReplaceAllString(s, t.Colorize.Error("$1")+"$2")
		s = regexpJSONErrors.ReplaceAllString(s, "$1"+t.Colorize.Error("$2")+"$3")
		s = regexpJSONOptimal.ReplaceAllString(s, "$1"+t.Colorize.Optimal("$2")+"$3")
		s = regexpJSONWarning.ReplaceAllString(s, "$1"+t.Colorize.Warning("$2")+"$3")
		s = regexpJSONSecondary.ReplaceAllString(s, "$1"+t.Colorize.Secondary("$2")+"$3")
		return s
	case JSONLine:
		b, err := json.Marshal(t.Data)
		if err != nil {
			panic(err)
		}
		return string(b) + "\n"
	case YAML:
		var sep string
		if t.Stream {
			sep = "---\n"
		}
		if color.NoColor {
			b, err := yaml.Marshal(t.Data)
			if err != nil {
				panic(err)
			}
			return string(b) + sep
		} else {
			b := bytes.NewBuffer(nil)
			enc := yaml.NewEncoder(b)
			err := enc.Encode(t.Data)
			if err != nil {
				panic(err)
			}
			s, err := highlight.Highlight(b)
			if err != nil {
				panic(err)
			}
			return s + sep
		}
	default:
		if t.HumanRenderer != nil {
			return t.HumanRenderer()
		}
		if r, ok := t.Data.(renderer); ok {
			return r.Render()
		}
		b, err := json.MarshalIndent(t.Data, "", indent)
		if err != nil {
			panic(err)
		}
		return string(b) + "\n"
	}
}

// Print prints the representation of the data in one of the
// supported format (json, flat, human, ...).
//
// The human format needs a RenderFunc to be passed.
func (t Renderer) Print() {
	fmt.Print(t.Sprint())
}
