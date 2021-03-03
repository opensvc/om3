package output

import (
	"encoding/json"
	"fmt"

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
		b, _ := json.MarshalIndent(t.Data, "", "    ")
		return string(b) + "\n"
	case JSONLine:
		b, _ := json.Marshal(t.Data)
		return string(b) + "\n"
	default:
		if t.HumanRenderer != nil {
			return t.HumanRenderer()
		}
		b, _ := json.MarshalIndent(t.Data, "", "    ")
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
