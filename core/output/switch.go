package output

import (
	"encoding/json"

	"opensvc.com/opensvc/util/render"
)

// RenderFunc must be passed by the Switch() caller
type RenderFunc func() string

// Switch returns the dataset in one of the supported format (json, flat, human, ...).
// The human format needs a RenderFunc to be passed.
func Switch(formatStr string, color string, data interface{}, human RenderFunc) string {
	format := toID[formatStr]
	render.SetColor(color)
	switch format {
	case Flat:
		b, _ := json.Marshal(data)
		return SprintFlat(b)
	case JSON:
		b, _ := json.MarshalIndent(data, "", "    ")
		return string(b) + "\n"
	case JSONLine:
		b, _ := json.Marshal(data)
		return string(b) + "\n"
	default:
		if human != nil {
			return human()
		}
		b, _ := json.MarshalIndent(data, "", "    ")
		return string(b) + "\n"
	}
}
