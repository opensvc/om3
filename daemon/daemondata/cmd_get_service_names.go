package daemondata

import "context"

type opGetServiceNames struct {
	services chan<- []string
}

func (o opGetServiceNames) call(ctx context.Context, d *data) {

	paths := make(map[string]bool)
	for node := range d.committed.Monitor.Nodes {
		for s := range d.committed.Monitor.Nodes[node].Services.Config {
			paths[s] = true
		}
	}
	services := make([]string, 0)
	for s := range paths {
		services = append(services, s)
	}
	select {
	case <-ctx.Done():
	case o.services <- services:
	}
}

// GetServiceNames returns service names from cluster nodes dataset config
//
// committed.Monitor.Nodes[*].Services.Config[*]
func (t T) GetServiceNames() []string {
	services := make(chan []string)
	t.cmdC <- opGetServiceNames{
		services: services,
	}
	return <-services
}
