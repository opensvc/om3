package nmon

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/key"
)

type (
	arbitratorConfig struct {
		Name     string `json:"name"`
		Uri      string `json:"uri"`
		Insecure bool
	}
)

// setArbitratorConfig load config to sets arbitrators
func (o *nmon) setArbitratorConfig() {
	arbitrators := make(map[string]arbitratorConfig)
	for _, s := range o.config.SectionStrings() {
		if !strings.HasPrefix(s, "arbitrator#") {
			continue
		}
		name := strings.TrimPrefix(s, "arbitrator#")
		a := arbitratorConfig{
			Name:     name,
			Uri:      o.config.GetString(key.New(s, "uri")),
			Insecure: o.config.GetBool(key.New(s, "insecure")),
		}
		if a.Uri == "" {
			o.log.Debug().Msgf("arbitrator keyword 'name' is deprecated, use 'uri' instead")
			a.Uri = o.config.GetString(key.New(s, "name"))
		}
		if a.Uri == "" {
			o.log.Warn().Msgf("ignored arbitrator %s (empty uri)", s)
			continue
		}
		arbitrators[name] = a
	}
	o.arbitrators = arbitrators
}

// getStatusArbitrators checks all arbitrators and returns result
func (o *nmon) getStatusArbitrators() map[string]node.ArbitratorStatus {
	type res struct {
		name string
		err  error
	}
	ctx, cancel := context.WithTimeout(o.ctx, arbitratorCheckDuration)
	defer cancel()
	c := make(chan res, len(o.arbitrators))
	for _, a := range o.arbitrators {
		go func(a arbitratorConfig) {
			c <- res{name: a.Name, err: o.arbitratorCheck(ctx, a)}
		}(a)
	}
	result := make(map[string]node.ArbitratorStatus)
	for i := 0; i < len(o.arbitrators); i++ {
		r := <-c
		name := r.name
		url := o.arbitrators[name].Uri
		aStatus := status.Up
		if r.err != nil {
			o.log.Warn().Msgf("arbitrator#%s is down", name)
			o.log.Debug().Err(r.err).Msgf("arbitrator#%s is down", name)
			aStatus = status.Down
			o.bus.Pub(&msgbus.ArbitratorError{
				Node: o.localhost,
				Name: name,
				ErrS: r.err.Error(),
			})
		}
		result[name] = node.ArbitratorStatus{Url: url, Status: aStatus}
	}
	return result
}

func (o *nmon) getAndUpdateStatusArbitrator() {
	o.nodeStatus.Arbitrators = o.getStatusArbitrators()
	o.bus.Pub(&msgbus.NodeStatusUpdated{Node: o.localhost, Value: *o.nodeStatus.DeepCopy()}, o.labelLocalhost)
	pubValue := make(map[string]node.ArbitratorStatus)
	for k, v := range o.nodeStatus.Arbitrators {
		pubValue[k] = v
	}
	o.bus.Pub(&msgbus.NodeStatusArbitratorsUpdated{Node: o.localhost, Value: pubValue}, o.labelLocalhost)
}

func (o *nmon) arbitratorVotes() (votes []string) {
	for s, v := range o.getStatusArbitrators() {
		if v.Status == status.Up {
			votes = append(votes, s)
		}
	}
	return
}

func (o *nmon) arbitratorCheck(ctx context.Context, a arbitratorConfig) error {
	if strings.HasPrefix(a.Uri, "http") {
		return a.checkUrl(ctx)
	}
	if a.Uri != "" {
		return a.checkDial(ctx)
	}
	return fmt.Errorf("invalid arbitrator uri")
}

func (a *arbitratorConfig) checkUrl(ctx context.Context) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: a.Insecure,
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", a.Uri, nil)
	if err != nil {
		return err
	}
	_, err = client.Do(req)
	return err
}

func (a *arbitratorConfig) checkDial(ctx context.Context) error {
	d := net.Dialer{}
	addr := a.Uri
	if !strings.Contains(addr, ":") {
		port := ccfg.Get().Listener.Port
		addr = fmt.Sprintf("%s:%d", addr, port)
	}
	dialContext, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return dialContext.Close()
}
