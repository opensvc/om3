package object

import (
	"context"
	"errors"
	"net"

	"github.com/opensvc/om3/v3/core/resourceselector"
)

type graphicsDialer interface {
	GraphicsDial(context.Context) (net.Conn, error)
}

// GraphicsDial connects to the display of the first resource the rid selector
// selects having one.
func (t *actor) GraphicsDial(ctx context.Context, rid string) (net.Conn, error) {
	rs := resourceselector.New(t, resourceselector.WithRID(rid))
	var dialer graphicsDialer
	for _, r := range rs.Resources() {
		if i, ok := r.(graphicsDialer); ok {
			dialer = i
			break
		}
	}
	if dialer == nil {
		return nil, errors.New("no resource has a display")
	}
	return dialer.GraphicsDial(ctx)
}
