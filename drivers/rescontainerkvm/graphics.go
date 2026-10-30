package rescontainerkvm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/antchfx/xmlquery"
)

// GraphicsDial connects to the display of the running guest, as the definition
// libvirt holds of it says where it is: on a socket, or on a port of its
// choosing, reached from this node. The session using it is authorized by the
// ticket it was opened with.
func (t *T) GraphicsDial(ctx context.Context) (net.Conn, error) {
	b, err := t.liveDefinition(ctx)
	if err != nil {
		return nil, fmt.Errorf("the guest is not running here: %w", err)
	}
	network, address, err := displayAddress(b)
	if err != nil {
		return nil, err
	}
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

// displayAddress returns where the vnc display of a domain definition listens.
// The browser speaks vnc only, so another display is said by name for its
// definition to be changed, rather than as a connection that fails.
func displayAddress(b []byte) (string, string, error) {
	doc, err := xmlquery.Parse(bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	nodes, err := xmlquery.QueryAll(doc, "//domain/devices/graphics")
	if err != nil {
		return "", "", err
	}
	other := ""
	for _, n := range nodes {
		if typ := n.SelectAttr("type"); typ != "vnc" {
			other = typ
			continue
		}
		if s := n.SelectAttr("socket"); s != "" {
			return "unix", s, nil
		}
		port, err := strconv.Atoi(n.SelectAttr("port"))
		if err != nil || port <= 0 {
			return "", "", errors.New("the vnc display of the guest has no port")
		}
		return "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
	}
	if other != "" {
		return "", "", fmt.Errorf("the guest has a %s display, and a vnc one is needed", other)
	}
	return "", "", errors.New("the guest has no display")
}
