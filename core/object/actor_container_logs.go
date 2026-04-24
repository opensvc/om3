package object

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/resourceselector"
)

type containerLogger interface {
	ContainerLogs(context.Context, bool, int) (<-chan []byte, error)
}

// ContainerLogs returns container logs
func (t *actor) ContainerLogs(ctx context.Context, rid string, follow bool, lines int) error {
	rs := resourceselector.New(t, resourceselector.WithRID(rid))
	var container containerLogger
	for _, r := range rs.Resources() {
		if i, ok := r.(containerLogger); !ok {
			continue
		} else if container != nil {
			return fmt.Errorf("multiple resources support container logs. use the --rid option")
		} else {
			container = i
			rid = r.RID()
		}
	}
	if container == nil {
		return fmt.Errorf("no resource supports container logs")
	}

	logChan, err := container.ContainerLogs(ctx, follow, lines)
	if err != nil {
		return fmt.Errorf("%s: %w", rid, err)
	}

	// Stream the logs
	for logData := range logChan {
		os.Stdout.Write(logData)
	}

	return nil
}

// ContainerLogsStream returns container logs as a stream
func (t *actor) ContainerLogsStream(ctx context.Context, rid string, follow bool, lines int) (<-chan []byte, error) {
	rs := resourceselector.New(t, resourceselector.WithRID(rid))
	var container containerLogger
	for _, r := range rs.Resources() {
		if i, ok := r.(containerLogger); !ok {
			continue
		} else if container != nil {
			return nil, fmt.Errorf("multiple resources support container logs. use the --rid option")
		} else {
			container = i
			rid = r.RID()
		}
	}
	if container == nil {
		return nil, fmt.Errorf("no resource supports container logs")
	}

	return container.ContainerLogs(ctx, follow, lines)
}
