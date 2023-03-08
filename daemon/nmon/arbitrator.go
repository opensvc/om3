package nmon

import (
	"crypto/tls"
	"net/http"
	"strings"
	"time"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/key"
)

type (
	arbitratorConfig struct {
		Name     string `json:"name"`
		Url      string `json:"url"`
		Insecure bool

		timeout time.Duration
		secret  string
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
			Url:      o.config.GetString(key.New(s, "url")),
			Insecure: o.config.GetBool(key.New(s, "insecure")),
		}
		if d := o.config.GetDuration(key.New(s, "timeout")); d != nil {
			a.timeout = *d
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
	c := make(chan res, len(o.arbitrators))
	for _, a := range o.arbitrators {
		go func(a arbitratorConfig) {
			c <- res{name: a.Name, err: o.arbitratorCheck(a)}
		}(a)
	}
	result := make(map[string]node.ArbitratorStatus)
	for i := 0; i < len(o.arbitrators); i++ {
		r := <-c
		name := r.name
		url := o.arbitrators[name].Url
		aStatus := status.Up
		if r.err != nil {
			o.log.Warn().Msgf("arbitrator#%s is down", name)
			o.log.Debug().Err(r.err).Msgf("arbitrator#%s is down", name)
			aStatus = status.Down
			o.bus.Pub(msgbus.ArbitratorError{
				Node: o.localhost,
				Name: name,
				Err:  r.err,
			})
		}
		result[name] = node.ArbitratorStatus{Url: url, Status: aStatus}
	}
	return result
}

func (o *nmon) getAndUpdateStatusArbitrator() error {
	a := o.getStatusArbitrators()
	return o.databus.SetNodeStatusArbitrator(a)
}

func (o *nmon) arbitratorVotes() (votes []string) {
	for s, v := range o.getStatusArbitrators() {
		if v.Status == status.Up {
			votes = append(votes, s)
		}
	}
	return
}

func (o *nmon) arbitratorCheck(a arbitratorConfig) error {
	client := &http.Client{
		Timeout: a.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: a.Insecure,
			},
		},
	}
	if req, err := http.NewRequestWithContext(o.ctx, "GET", a.Url, nil); err != nil {
		return err
	} else {
		_, err = client.Do(req)
		return err
	}
}
