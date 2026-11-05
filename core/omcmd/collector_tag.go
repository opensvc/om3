package omcmd

import (
	"errors"
	"fmt"

	"github.com/opensvc/om3/v3/core/collector"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

// newCollectorTagClient returns a client of the tag routes of the collector,
// authenticated as this node.
func newCollectorTagClient() (*collector.TagClient, error) {
	n, err := object.NewNode()
	if err != nil {
		return nil, err
	}
	cli, err := n.CollectorServer()
	if err != nil {
		return nil, err
	}
	return collector.NewTagClient(cli), nil
}

func collectorTagCheckName(name string) error {
	if name == "" {
		return errors.New("the tag name must not be empty")
	}
	return nil
}

// collectorTagShow returns the tags attached to the object at p, or to the
// node if p is zero: their names, or all their data with verbose.
func collectorTagShow(p naming.Path, verbose bool) (any, error) {
	cli, err := newCollectorTagClient()
	if err != nil {
		return nil, err
	}
	l, err := cli.Attached(p)
	if err != nil {
		return nil, err
	}
	if verbose {
		return l, nil
	}
	names := make([]string, len(l))
	for i, e := range l {
		names[i] = e.TagName
	}
	return names, nil
}

func collectorTagList() (any, error) {
	cli, err := newCollectorTagClient()
	if err != nil {
		return nil, err
	}
	return cli.Names()
}

func collectorTagCreate(name string, data, exclude *string) (any, error) {
	if err := collectorTagCheckName(name); err != nil {
		return nil, err
	}
	cli, err := newCollectorTagClient()
	if err != nil {
		return nil, err
	}
	return printInfo(cli.Create(name, data, exclude))
}

func collectorTagAttach(name string, p naming.Path, attachData *string) (any, error) {
	if err := collectorTagCheckName(name); err != nil {
		return nil, err
	}
	cli, err := newCollectorTagClient()
	if err != nil {
		return nil, err
	}
	return printInfo(cli.Attach(name, p, attachData))
}

func collectorTagDetach(name string, p naming.Path) (any, error) {
	if err := collectorTagCheckName(name); err != nil {
		return nil, err
	}
	cli, err := newCollectorTagClient()
	if err != nil {
		return nil, err
	}
	return printInfo(cli.Detach(name, p))
}

// printInfo prints the answer of the collector to a change, the commands
// returning no data.
func printInfo(info string, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	fmt.Println(info)
	return nil, nil
}
