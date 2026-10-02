// Package restaskacme is the task.acme driver: a task renewing the
// certificates of secs, when due, from the ACME directory each names, or as
// certificate create generates them.
//
// Its action is om's own, rather than a command its writer chooses, so a
// namespace administrator may configure it: it writes the certificates of the
// secs its namespace may use, and on the node only the http-01 challenge
// tokens, inside a volume of its object.
package restaskacme

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/drivers/restask"
	"github.com/opensvc/om3/v3/util/confined"
)

type (
	T struct {
		restask.BaseTask
		Path    naming.Path    `json:"path"`
		Secs    []string       `json:"secs"`
		Webroot string         `json:"webroot"`
		Timeout *time.Duration `json:"timeout"`
	}

	// confinedWebroot writes the http-01 challenge tokens in a directory of
	// a volume, never outside the volume: a link planted in it by what the
	// volume is mounted in does not lead the write elsewhere.
	confinedWebroot struct {
		tree *confined.Tree
		dir  string
	}

	resourceByIDer interface {
		ResourceByID(string) resource.Driver
	}

	header interface {
		Head() string
	}
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Label(_ context.Context) string {
	return "acme " + strings.Join(t.Secs, " ")
}

func (t *T) Run(ctx context.Context) error {
	return t.RunIf(ctx, t.lockedRun)
}

// lockedRun renews the certificate of each sec, those after a failed one
// included, and says which failed.
func (t *T) lockedRun(ctx context.Context) error {
	if t.Timeout != nil && *t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.Timeout)
		defer cancel()
	}
	provider, closeProvider, err := t.http01()
	if err != nil {
		_ = t.WriteLastRun(1)
		return err
	}
	defer closeProvider()
	var errs error
	for _, ref := range t.Secs {
		if err := t.renew(ctx, ref, provider); err != nil {
			t.Log().Errorf("%s: %s", ref, err)
			errs = errors.Join(errs, fmt.Errorf("%s: %w", ref, err))
		}
	}
	exitCode := 0
	if errs != nil {
		exitCode = 1
	}
	if err := t.WriteLastRun(exitCode); err != nil {
		return errors.Join(errs, err)
	}
	return errs
}

func (t *T) renew(ctx context.Context, ref string, provider challenge.Provider) error {
	p, err := secPath(ref, t.Path.Namespace)
	if err != nil {
		return err
	}
	if !p.Exists() {
		return fmt.Errorf("%s does not exist", p)
	}
	store, err := object.NewDataStore(p)
	if err != nil {
		return err
	}
	if p.Namespace != t.Path.Namespace && !store.Allow(t.Path.Namespace) {
		return fmt.Errorf("%s is not shared with namespace %s: its share keyword does not name it", p, t.Path.Namespace)
	}
	keyStore, ok := store.(object.KeyStore)
	if !ok {
		return fmt.Errorf("%s holds no certificate", p)
	}
	renewal, err := keyStore.RenewCertificate(ctx, object.CertificateRenewOptions{HTTP01: provider})
	if err != nil {
		return err
	}
	if renewal.Renewed {
		t.Log().Infof("%s: certificate issued for %s: %s", p, strings.Join(renewal.Domains, " "), renewal.Reason)
	} else {
		t.Log().Infof("%s: no certificate issued: %s", p, renewal.Reason)
	}
	return nil
}

// secPath returns the sec a reference names: a name or ./sec/<name> in the
// namespace ns, <namespace>/sec/<name> in another.
func secPath(ref, ns string) (naming.Path, error) {
	if !strings.Contains(ref, "/") {
		return naming.NewPath(ns, naming.KindSec, ref)
	}
	p, err := naming.ParsePathRel(ref, ns)
	if err != nil {
		return p, fmt.Errorf("sec reference %s: %w", ref, err)
	}
	if p.Kind != naming.KindSec {
		return p, fmt.Errorf("sec reference %s names a %s, not a sec", ref, p.Kind)
	}
	return p, nil
}

// http01 returns the writer of the http-01 challenge tokens the webroot
// keyword names, nil when it names none, and what closes it.
func (t *T) http01() (challenge.Provider, func(), error) {
	noop := func() {}
	if t.Webroot == "" {
		return nil, noop, nil
	}
	rid, sub, ok := strings.Cut(t.Webroot, ":")
	if !ok || rid == "" {
		return nil, noop, fmt.Errorf("webroot %s: expected <volume rid>:<path>", t.Webroot)
	}
	o, ok := t.GetObject().(resourceByIDer)
	if !ok {
		return nil, noop, fmt.Errorf("webroot %s: the object has no resources", t.Webroot)
	}
	r := o.ResourceByID(rid)
	if r == nil {
		return nil, noop, fmt.Errorf("webroot %s: no resource %s", t.Webroot, rid)
	}
	h, ok := r.(header)
	if !ok {
		return nil, noop, fmt.Errorf("webroot %s: resource %s is not a volume", t.Webroot, rid)
	}
	head := h.Head()
	if head == "" {
		return nil, noop, fmt.Errorf("webroot %s: volume %s has no mount point here", t.Webroot, rid)
	}
	tree, err := confined.Open(head)
	if err != nil {
		return nil, noop, fmt.Errorf("webroot %s: %w", t.Webroot, err)
	}
	dir := filepath.Join(head, filepath.Clean("/"+sub))
	return &confinedWebroot{tree: tree, dir: dir}, func() { _ = tree.Close() }, nil
}

func (t *confinedWebroot) tokenPath(token string) (string, error) {
	if token == "" || strings.ContainsAny(token, "/\\") || token == "." || token == ".." {
		return "", fmt.Errorf("unexpected challenge token %q", token)
	}
	return filepath.Join(t.dir, ".well-known", "acme-challenge", token), nil
}

// Present writes the challenge token, readable by the http server.
func (t *confinedWebroot) Present(_, token, keyAuth string) error {
	p, err := t.tokenPath(token)
	if err != nil {
		return err
	}
	if err := t.tree.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("make the challenge directory: %w", err)
	}
	if err := t.tree.WriteFile(p, []byte(keyAuth), 0o644); err != nil {
		return fmt.Errorf("write the challenge token: %w", err)
	}
	return nil
}

// CleanUp removes the challenge token.
func (t *confinedWebroot) CleanUp(_, token, _ string) error {
	p, err := t.tokenPath(token)
	if err != nil {
		return err
	}
	return t.tree.Remove(p)
}
