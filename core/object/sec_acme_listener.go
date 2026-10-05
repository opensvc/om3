package object

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// listenerHTTP01 writes the http-01 challenge tokens of a sec of
	// listener.tls_secs as keys of that sec, which the daemon replicates
	// to every node, whose listener answers them on listener.acme_port.
	listenerHTTP01 struct {
		sec   *sec
		nodes []string
		port  int
		log   *plog.Logger

		// mu orders the writes of the tokens of several domains, each a
		// commit of the configuration of the sec.
		mu sync.Mutex
	}
)

const (
	// acmeChallengeKeyPrefix names the keys holding the http-01 challenge
	// tokens of a renewal through the listener.
	acmeChallengeKeyPrefix = "acme_challenge/"

	// listenerHTTP01Wait bounds the wait for every node to answer a token,
	// the time the daemon takes to replicate the sec.
	listenerHTTP01Wait = time.Minute

	listenerHTTP01Poll = time.Second
)

// AcmeChallengeKey returns the key of the sec holding the key authorization
// of the http-01 challenge token, and refuses a token which is not one: a
// token is a base64url string, so it names no other key.
func AcmeChallengeKey(token string) (string, error) {
	if token == "" || len(token) > 256 {
		return "", fmt.Errorf("invalid challenge token")
	}
	for _, c := range token {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return "", fmt.Errorf("invalid challenge token")
		}
	}
	return acmeChallengeKeyPrefix + token, nil
}

// ListenerTLSSecs returns the secs listener.tls_secs names, as paths, or by
// their names in the system namespace. A reference naming no sec is left
// out.
func ListenerTLSSecs(refs []string) []naming.Path {
	l := make([]naming.Path, 0, len(refs))
	for _, ref := range refs {
		p, err := naming.ParsePathRel(ref, naming.NsSys)
		if err != nil || p.Kind != naming.KindSec {
			if p, err = naming.NewPath(naming.NsSys, naming.KindSec, ref); err != nil {
				continue
			}
		}
		if !slices.Contains(l, p) {
			l = append(l, p)
		}
	}
	return l
}

// newListenerHTTP01 returns the writer of the challenge tokens of the sec
// through the listener, and nil when the listener serves no certificate of
// it, or answers no challenge.
func (t *sec) newListenerHTTP01() (*listenerHTTP01, error) {
	cfg, err := getClusterConfig()
	if err != nil {
		return nil, err
	}
	if !slices.Contains(ListenerTLSSecs(cfg.Listener.TLSSecs), t.path) {
		return nil, nil
	}
	if cfg.Listener.ACMEPort <= 0 {
		return nil, fmt.Errorf("the listener presents the certificate of %s, and answers no ACME challenge: set listener.acme_port", t.path)
	}
	return &listenerHTTP01{
		sec:   t,
		nodes: cfg.Nodes,
		port:  cfg.Listener.ACMEPort,
		log:   t.Log(),
	}, nil
}

// Present stores the key authorization of the token in the sec, and returns
// once every node answering on listener.acme_port answers it: the ACME
// directory reads the token from whichever node the domain resolves to.
func (t *listenerHTTP01) Present(domain, token, keyAuth string) error {
	k, err := AcmeChallengeKey(token)
	if err != nil {
		return err
	}
	t.mu.Lock()
	err = t.sec.ChangeKey(k, []byte(keyAuth))
	t.mu.Unlock()
	if err != nil {
		return fmt.Errorf("store the challenge token: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), listenerHTTP01Wait)
	defer cancel()
	var errs error
	for _, node := range t.nodes {
		if err := t.waitServed(ctx, node, token, keyAuth); err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

// CleanUp removes the token from the sec.
func (t *listenerHTTP01) CleanUp(domain, token, keyAuth string) error {
	k, err := AcmeChallengeKey(token)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.sec.HasKey(k) {
		return nil
	}
	return t.sec.RemoveKey(k)
}

// waitServed waits for the listener of the node to answer the token. A node
// whose listener can not be reached is not waited for: it may be down, and
// the directory reaching it fails the renewal anyway.
func (t *listenerHTTP01) waitServed(ctx context.Context, node, token, keyAuth string) error {
	url := "http://" + net.JoinHostPort(node, strconv.Itoa(t.port)) + "/.well-known/acme-challenge/" + token
	client := &http.Client{Timeout: 5 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			var netErr *net.OpError
			if errors.As(err, &netErr) && netErr.Op == "dial" {
				t.log.Warnf("%s: the challenge listener can not be reached, not waiting for it to answer the token: %s", node, err)
				return nil
			}
		} else {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && bytes.Equal(bytes.TrimSpace(b), []byte(keyAuth)) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s does not answer the challenge token after %s", node, listenerHTTP01Wait)
		case <-time.After(listenerHTTP01Poll):
		}
	}
}
