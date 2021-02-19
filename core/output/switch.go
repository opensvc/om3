package output

import (
	"encoding/json"
	"fmt"

	"opensvc.com/opensvc/core/render"
)

// RenderFunc must be passed by the Switch() caller
type RenderFunc func()

// Switch outputs the dataset in one of the supported format (json, flat, human, ...).
// The human format needs a RenderFunc to be passed.
func Switch(formatStr string, color string, data interface{}, human RenderFunc) {
	format := toID[formatStr]
	render.SetColor(color)
	switch format {
	case Flat:
		b, _ := json.MarshalIndent(data, "", "    ")
		PrintFlat(b)
	case JSON:
		b, _ := json.MarshalIndent(data, "", "    ")
		fmt.Println(string(b))
	default:
		human()
	}
}
