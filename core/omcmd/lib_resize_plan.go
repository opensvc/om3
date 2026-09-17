package omcmd

import (
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

// printResizePlan reports what a resize would do, in the format asked for.
//
// A dry run answers a question rather than changing anything, so its answer
// goes through the renderer like any other: the steps and the refusal are
// fields a script can read, and not only a paragraph to look at.
func printResizePlan(plan object.ResizePlan, outputFormat, sort, color string) error {
	return output.Renderer{
		HumanRenderer: func() string {
			return plan.String() + "\n"
		},
		Output:   outputFormat,
		Sort:     sort,
		Color:    color,
		Data:     plan,
		Colorize: rawconfig.Colorize,
	}.Print()
}
