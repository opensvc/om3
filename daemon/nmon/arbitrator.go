package nmon

import (
	"context"
	"crypto/tls"
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	arbitratorConfig struct {
		Name     string `json:"name"`
		URI      string `json:"uri"`
		Weight   int    `json:"weight"`
		Insecure bool
	}
)

// setArbitratorConfig load config to sets arbitrators
func (t *Manager) setArbitratorConfig() {
	arbitrators := make(map[string]arbitratorConfig)
	for _, s := range t.config.SectionStrings() {
		if !strings.HasPrefix(s, "arbitrator#") {
			continue
		}
		name := strings.TrimPrefix(s, "arbitrator#")
		a := arbitratorConfig{
			Name:     name,
			URI:      t.config.GetString(key.New(s, "uri")),
			Insecure: t.config.GetBool(key.New(s, "insecure")),
			Weight:   t.config.GetInt(key.New(s, "weight")),
		}
		if a.URI == "" {
			t.log.Tracef("arbitrator keyword 'name' is deprecated, use 'uri' instead")
			a.URI = t.config.GetString(key.New(s, "name"))
		}
		if a.URI == "" {
			t.log.Warnf("ignored arbitrator %s (empty uri)", s)
			continue
		}
		arbitrators[name] = a
	}
	t.arbitrators = arbitrators
}

// getStatusArbitrators checks all arbitrators and returns result, waiting
// for the answers: the split decision is the one place a fresh answer is
// worth holding the monitor for.
func (t *Manager) getStatusArbitrators() map[string]node.ArbitratorStatus {
	return t.checkArbitrators(t.arbitrators)
}

// checkArbitrators checks the arbitrators given, a copy of the configuration
// for a check running out of the loop.
func (t *Manager) checkArbitrators(arbitrators map[string]arbitratorConfig) map[string]node.ArbitratorStatus {
	type res struct {
		name string
		err  error
	}
	ctx, cancel := context.WithTimeout(t.ctx, arbitratorCheckDuration)
	defer cancel()
	c := make(chan res, len(arbitrators))
	for _, a := range arbitrators {
		go func(a arbitratorConfig) {
			c <- res{name: a.Name, err: t.arbitratorCheck(ctx, a)}
		}(a)
	}
	result := make(map[string]node.ArbitratorStatus)
	for i := 0; i < len(arbitrators); i++ {
		r := <-c
		name := r.name
		aStatus := status.Up
		if r.err != nil {
			t.log.Warnf("arbitrator#%s is down", name)
			t.log.Tracef("arbitrator#%s is down: %s", name, r.err)
			aStatus = status.Down
			t.publisher.Pub(&msgbus.ArbitratorError{
				Node: t.localhost,
				Name: name,
				ErrS: r.err.Error(),
			})
		}
		result[name] = node.ArbitratorStatus{
			URL:    arbitrators[name].URI,
			Status: aStatus,
			Weight: arbitrators[name].Weight,
		}
	}
	return result
}

// refreshArbitrators checks the arbitrators out of the loop, the result
// coming back through arbitratorC: an arbitrator not answering held the
// monitor for the whole timeout of the check, at the daemon start, every
// arbitratorInterval and at every configuration change, and a request the
// api handed the monitor meanwhile timed out. A check asked while one runs
// runs once it ends, the configuration having changed.
func (t *Manager) refreshArbitrators() {
	if t.arbitratorChecking {
		t.arbitratorRecheck = true
		return
	}
	t.arbitratorChecking = true
	arbitrators := maps.Clone(t.arbitrators)
	go func() {
		result := t.checkArbitrators(arbitrators)
		select {
		case t.arbitratorC <- result:
		case <-t.ctx.Done():
		}
	}()
}

// onArbitratorsChecked records what a background check found.
func (t *Manager) onArbitratorsChecked(arbitrators map[string]node.ArbitratorStatus) {
	t.arbitratorChecking = false
	if t.arbitratorRecheck {
		t.arbitratorRecheck = false
		t.refreshArbitrators()
		return
	}
	t.updateStatusArbitrator(arbitrators)
}

func (t *Manager) updateStatusArbitrator(arbitrators map[string]node.ArbitratorStatus) {
	if maps.Equal(arbitrators, t.nodeStatus.Arbitrators) {
		return
	}
	t.nodeStatus.Arbitrators = arbitrators
	t.publishNodeStatus()
	pubValue := make(map[string]node.ArbitratorStatus)
	for k, v := range t.nodeStatus.Arbitrators {
		pubValue[k] = v
	}
	t.publisher.Pub(&msgbus.NodeStatusArbitratorsUpdated{Node: t.localhost, Value: pubValue}, t.labelLocalhost)
}

// arbitratorTotal is the weight of the arbitrators, which their
// configuration says, with no need to reach them.
func (t *Manager) arbitratorTotal() int {
	i := 0
	for _, a := range t.arbitrators {
		i += a.Weight
	}
	return i
}

func (t *Manager) arbitratorVotes() (votes []string) {
	for s, v := range t.getStatusArbitrators() {
		if v.Status == status.Up {
			for i := 0; i < v.Weight; i++ {
				votes = append(votes, s)
			}
		}
	}
	return
}

func (t *Manager) arbitratorCheck(ctx context.Context, a arbitratorConfig) error {
	if strings.HasPrefix(a.URI, "http") {
		return a.checkURL(ctx)
	}
	if a.URI != "" {
		return a.checkDial(ctx)
	}
	return fmt.Errorf("invalid arbitrator uri")
}

func (a *arbitratorConfig) checkURL(ctx context.Context) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: a.Insecure,
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", a.URI, nil)
	if err != nil {
		return err
	}
	_, err = client.Do(req)
	return err
}

func (a *arbitratorConfig) checkDial(ctx context.Context) error {
	d := net.Dialer{}
	addr := a.URI
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, cluster.ConfigData.Get().Listener.Port)
	}
	dialContext, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return dialContext.Close()
}
