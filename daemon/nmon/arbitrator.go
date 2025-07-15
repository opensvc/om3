package nmon

import (
	"context"
	"crypto/tls"
	"fmt"
	"maps"
	"net"
	"net/http"
	"strings"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/key"
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
			t.log.Debugf("arbitrator keyword 'name' is deprecated, use 'uri' instead")
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

// getStatusArbitrators checks all arbitrators and returns result
func (t *Manager) getStatusArbitrators() map[string]node.ArbitratorStatus {
	type res struct {
		name string
		err  error
	}
	ctx, cancel := context.WithTimeout(t.ctx, arbitratorCheckDuration)
	defer cancel()
	c := make(chan res, len(t.arbitrators))
	for _, a := range t.arbitrators {
		go func(a arbitratorConfig) {
			c <- res{name: a.Name, err: t.arbitratorCheck(ctx, a)}
		}(a)
	}
	result := make(map[string]node.ArbitratorStatus)
	for i := 0; i < len(t.arbitrators); i++ {
		r := <-c
		name := r.name
		aStatus := status.Up
		if r.err != nil {
			t.log.Warnf("arbitrator#%s is down", name)
			t.log.Debugf("arbitrator#%s is down: %s", name, r.err)
			aStatus = status.Down
			t.publisher.Pub(&msgbus.ArbitratorError{
				Node: t.localhost,
				Name: name,
				ErrS: r.err.Error(),
			})
		}
		result[name] = node.ArbitratorStatus{
			URL:    t.arbitrators[name].URI,
			Status: aStatus,
			Weight: t.arbitrators[name].Weight,
		}
	}
	return result
}

func (t *Manager) getAndUpdateStatusArbitrator() {
	arbitrators := t.getStatusArbitrators()
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

func (t *Manager) arbitratorTotal() int {
	i := 0
	for _, v := range t.getStatusArbitrators() {
		i += v.Weight
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
