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
	"slices"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/drivers/restask"
	"github.com/opensvc/om3/v3/util/confined"
)

type (
	T struct {
		restask.BaseTask
		Path        naming.Path    `json:"path"`
		Secs        []string       `json:"secs"`
		Webroot     string         `json:"webroot"`
		Directory   string         `json:"directory"`
		Email       string         `json:"email"`
		DomainsFrom string         `json:"domains_from"`
		DomainsTo   string         `json:"domains_to"`
		Timeout     *time.Duration `json:"timeout"`
	}

	// confinedWebroot writes the http-01 challenge tokens in a directory of
	// a volume, never outside the volume: a link planted in it by what the
	// volume is mounted in does not lead the write elsewhere.
	confinedWebroot struct {
		tree *confined.Tree
		dir  string
	}
)

func New() resource.Driver {
	return &T{}
}

// Label is the secs renewed, and where the domains are read from: the status
// says the driver already.
func (t *T) Label(_ context.Context) string {
	l := slices.Clone(t.Secs)
	if t.DomainsFrom != "" {
		l = append(l, t.DomainsFrom)
	}
	return strings.Join(l, " ")
}

// checkDomains refuses domains_from without domains_to, and the other way
// around: the domains are read from the one, and their secs named by the
// other, so either alone obtains no certificate.
func (t *T) checkDomains() error {
	switch {
	case t.DomainsFrom != "" && t.DomainsTo == "":
		return fmt.Errorf("domains_from is set and domains_to is not: set domains_to, the secs of the certificates of the domains")
	case t.DomainsFrom == "" && t.DomainsTo != "":
		return fmt.Errorf("domains_to is set and domains_from is not: set domains_from, where the domains are read from")
	}
	return nil
}

func (t *T) Run(ctx context.Context) error {
	return t.RunIf(ctx, t.lockedRun)
}

// lockedRun renews the certificate of each sec, those after a failed one
// included, and says which failed. The secs of the domains are created and
// deleted first, and renewed with the secs named, all of them sharing one
// account at the directory.
func (t *T) lockedRun(ctx context.Context) error {
	if err := t.RunInstall(ctx); err != nil {
		return err
	}
	if err := t.checkDomains(); err != nil {
		_ = t.WriteLastRun(1)
		return err
	}
	if t.Timeout != nil && *t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *t.Timeout)
		defer cancel()
	}
	provider, closeProvider, err := t.http01(ctx)
	if err != nil {
		_ = t.WriteLastRun(1)
		return err
	}
	defer closeProvider()
	var errs error
	if len(t.Secs) == 0 && t.DomainsFrom == "" {
		_ = t.WriteLastRun(1)
		return fmt.Errorf("no sec to renew: set secs, or domains_from and domains_to")
	}
	paths, err := t.secs()
	if err != nil {
		t.Log().Errorf("%s", err)
		errs = err
	}
	accounts := slices.Clone(paths)
	if t.DomainsFrom != "" {
		renew, all, err := t.syncDomains(ctx)
		if err != nil {
			errs = errors.Join(errs, err)
		}
		paths = append(paths, renew...)
		accounts = append(accounts, all...)
	}
	slices.SortFunc(accounts, func(a, b naming.Path) int { return strings.Compare(a.String(), b.String()) })
	for _, p := range paths {
		if err := t.renew(ctx, p, provider, accounts); err != nil {
			t.Log().Errorf("%s: %s", p, err)
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
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

// secs returns the secs the task renews, each once: the ones the secs keyword
// names, and the ones its name patterns match on the node.
//
// A sec named is one asked: one that does not exist, or is not shared with
// the namespace of the service, is an error. A pattern names the secs there
// are, as the ones of the domains a gateway creates and deletes with its
// routes: matching none, it names none, and it leaves out the secs of
// another namespace not shared with this one.
func (t *T) secs() (naming.Paths, error) {
	ns := t.Path.Namespace
	l := make(naming.Paths, 0)
	add := func(p naming.Path) {
		if !slices.Contains(l, p) {
			l = append(l, p)
		}
	}
	var errs error
	for _, ref := range t.Secs {
		if naming.IsPathPattern(ref) {
			pattern, err := secPattern(ref, ns)
			if err != nil {
				errs = errors.Join(errs, err)
				continue
			}
			for _, p := range pattern.InstalledPaths() {
				if p.Namespace != ns && !allows(p, ns) {
					continue
				}
				add(p)
			}
			continue
		}
		p, err := secPath(ref, ns)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		if !p.Exists() {
			errs = errors.Join(errs, fmt.Errorf("%s does not exist", p))
			continue
		}
		if p.Namespace != ns && !allows(p, ns) {
			errs = errors.Join(errs, fmt.Errorf("%s is not shared with namespace %s: its share keyword does not name it", p, ns))
			continue
		}
		add(p)
	}
	return l, errs
}

// allows says whether the sec at p shares its keys with the namespace ns.
func allows(p naming.Path, ns string) bool {
	store, err := object.NewDataStore(p, object.WithVolatile(true))
	return err == nil && store.Allow(ns)
}

func (t *T) renew(ctx context.Context, p naming.Path, provider challenge.Provider, accounts naming.Paths) error {
	store, err := object.NewDataStore(p)
	if err != nil {
		return err
	}
	keyStore, ok := store.(object.KeyStore)
	if !ok {
		return fmt.Errorf("%s holds no certificate", p)
	}
	renewal, err := keyStore.RenewCertificate(ctx, object.CertificateRenewOptions{HTTP01: provider, Accounts: accounts})
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

// secPattern returns the secs a pattern of their name names, as
// ./sec/haproxy-*, relative to the namespace ns as a sec reference is.
func secPattern(ref, ns string) (naming.PathPattern, error) {
	if !strings.Contains(ref, "/") {
		ref = "./sec/" + ref
	}
	p, err := naming.ParsePathPatternRel(ref, ns)
	if err != nil {
		return p, fmt.Errorf("sec reference %s: %w", ref, err)
	}
	if p.Kind != naming.KindSec {
		return p, fmt.Errorf("sec reference %s names %s objects, not secs", ref, p.Kind)
	}
	return p, nil
}

// validateSecs says whether each word of the secs keyword names a sec, or
// secs by a pattern of their name. The namespace is any: a relative
// reference is checked as the one of the service would be.
func validateSecs(value string) error {
	var errs error
	for _, ref := range strings.Fields(value) {
		var err error
		if naming.IsPathPattern(ref) {
			_, err = secPattern(ref, "ns")
		} else {
			_, err = secPath(ref, "ns")
		}
		errs = errors.Join(errs, err)
	}
	return errs
}

// http01 returns the writer of the http-01 challenge tokens the webroot
// keyword names, nil when it names none, and what closes it.
//
// The webroot is a path in a volume or a filesystem of the service, as
// volume#1:/www or a vol name, never a path of the node: the task writes as
// root, and its writer may not be.
func (t *T) http01(ctx context.Context) (challenge.Provider, func(), error) {
	noop := func() {}
	if t.Webroot == "" {
		return nil, noop, nil
	}
	if strings.HasPrefix(t.Webroot, "/") {
		return nil, noop, fmt.Errorf("webroot %s: a path of the node is refused: name a volume or a filesystem of the service, as volume#1:/www", t.Webroot)
	}
	target, err := vpath.Resolve(ctx, t.Webroot, t.Path.Namespace, vpath.ResolverOf(t.GetObject()))
	if err != nil {
		return nil, noop, fmt.Errorf("webroot %s: %w", t.Webroot, err)
	}
	if target.Head == "" {
		return nil, noop, fmt.Errorf("webroot %s: not in a volume or a filesystem of the service", t.Webroot)
	}
	tree, err := confined.Open(target.Head)
	if err != nil {
		return nil, noop, fmt.Errorf("webroot %s: %w", t.Webroot, err)
	}
	return &confinedWebroot{tree: tree, dir: filepath.Clean(target.HostPath)}, func() { _ = tree.Close() }, nil
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
