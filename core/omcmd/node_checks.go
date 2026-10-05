package omcmd

import (
	"context"
	"errors"

	"github.com/opensvc/om3/v3/core/check"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdNodeChecks struct {
		OptsGlobal
		NodeSelector string
	}
)

// Run runs the checks, pushes their results to the collector, and lists
// them: also when the push failed, which the command then says.
func (t *CmdNodeChecks) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	rs, pushErr := n.Checks(context.Background())
	renderErr := output.Renderer{
		DefaultOutput: check.DefaultOutput,
		DefaultSort:   check.DefaultSort,
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          rs.Lines(),
		Colorize:      rawconfig.Colorize,
	}.Print()
	return errors.Join(renderErr, pushErr)
}
