package restaskacme

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/confined"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// domainsFrom is where the task reads the domains from: the files of a
	// directory of a volume, the part of their name the * matches being a
	// domain, as volume#1:/haproxy/conf.d/*.cfg.
	domainsFrom struct {
		dir    string
		prefix string
		suffix string
	}

	// domainSec is a sec the task created for a domain.
	domainSec struct {
		path   naming.Path
		domain string
	}

	resourceLister interface {
		Resources() resource.Drivers
	}
)

const (
	// keyAcmeOwner is the key of a sec the task created for a domain,
	// naming the task: the task deletes the secs it created only, never
	// another sec its domains_to pattern matches.
	keyAcmeOwner = "acme_owner"

	// maxCreatedPerRun is the number of secs of domains a run creates at
	// most. The certificates obtained are installed, and the install runs
	// the task again for the next ones: a flood of routes does not make a
	// flood of secs and orders at once.
	maxCreatedPerRun = 10

	// maxSecName is the length of an object name, as naming validates it.
	maxSecName = 63
)

var (
	domainRE = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)

	errDomainsFrom = errors.New("domains_from is a directory of a volume and a file name pattern holding one *, as volume#1:/haproxy/conf.d/*.cfg")
)

// isDomain says whether s is a domain name the task obtains a certificate
// for: lowercase dot-separated labels, ending with an alphabetic top level
// domain.
func isDomain(s string) bool {
	if len(s) > 253 || !domainRE.MatchString(s) {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if len(label) > 63 {
			return false
		}
	}
	return true
}

func parseDomainsFrom(s string) (domainsFrom, error) {
	var t domainsFrom
	i := strings.LastIndex(s, "/")
	if i < 0 || strings.Count(s, "*") != 1 || !strings.Contains(s[i+1:], "*") {
		return t, fmt.Errorf("%s: %w", s, errDomainsFrom)
	}
	t.dir = s[:i]
	if t.dir == "" || strings.HasSuffix(t.dir, ":") {
		t.dir += "/"
	}
	t.prefix, t.suffix, _ = strings.Cut(s[i+1:], "*")
	return t, nil
}

// match returns the domain the file name holds, none when the name does not
// match the pattern or the part matched is no domain.
func (t domainsFrom) match(name string) (string, bool) {
	if len(name) <= len(t.prefix)+len(t.suffix) {
		return "", false
	}
	if !strings.HasPrefix(name, t.prefix) || !strings.HasSuffix(name, t.suffix) {
		return "", false
	}
	d := name[len(t.prefix) : len(name)-len(t.suffix)]
	return d, isDomain(d)
}

func validateDomainsFrom(value string) error {
	_, err := parseDomainsFrom(value)
	return err
}

// validateDomainsTo says whether the value names secs by a pattern, in the
// namespace of the service: a pattern naming its namespace is checked when
// the task runs, which knows the one of the service.
func validateDomainsTo(value string) error {
	_, err := parseDomainsTo(value, "ns")
	return err
}

// parseDomainsTo returns the secs the domains_to keyword names: a pattern
// whose * the domain replaces, in the namespace ns.
func parseDomainsTo(s, ns string) (naming.PathPattern, error) {
	return secPattern(s, ns)
}

// domainSecPath returns the sec of a domain: the pattern, its * replaced by
// the domain, or by the beginning of the domain and a hash of it when the
// name would be longer than an object name is. The sec is told by the domain
// it is the certificate of, not by its name.
func domainSecPath(pattern naming.PathPattern, domain string) (naming.Path, error) {
	prefix, suffix, _ := strings.Cut(pattern.Name, "*")
	name := prefix + domain + suffix
	if len(name) > maxSecName {
		sum := sha256.Sum256([]byte(domain))
		h := hex.EncodeToString(sum[:])[:8]
		room := maxSecName - len(prefix) - len(suffix) - len(h) - 1
		if room < 1 {
			return naming.Path{}, fmt.Errorf("the pattern %s leaves no room for a domain in a sec name", pattern)
		}
		name = prefix + strings.TrimRight(domain[:room], ".-") + "-" + h + suffix
	}
	return naming.NewPath(pattern.Namespace, pattern.Kind, name)
}

// owner is what the task writes in the secs it creates.
func (t *T) owner() string {
	return t.Path.String() + " " + t.RID()
}

// listDomains returns the domains the files domains_from names hold, sorted.
func (t *T) listDomains(ctx context.Context, from domainsFrom) ([]string, error) {
	target, err := vpath.Resolve(ctx, from.dir, t.Path.Namespace, vpath.ResolverOf(t.GetObject()))
	if err != nil {
		return nil, fmt.Errorf("domains_from %s: %w", from.dir, err)
	}
	if target.Head == "" {
		return nil, fmt.Errorf("domains_from %s: not in a volume or a filesystem of the service", from.dir)
	}
	tree, err := confined.Open(target.Head)
	if err != nil {
		return nil, fmt.Errorf("domains_from %s: %w", from.dir, err)
	}
	defer func() { _ = tree.Close() }()
	dir, err := tree.Open(target.HostPath)
	if err != nil {
		return nil, fmt.Errorf("domains_from %s: %w", from.dir, err)
	}
	defer func() { _ = dir.Close() }()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("domains_from %s: %w", from.dir, err)
	}
	l := make([]string, 0)
	for _, name := range names {
		d, ok := from.match(name)
		if !ok || slices.Contains(l, d) {
			continue
		}
		if info, err := tree.Lstat(filepath.Join(target.HostPath, name)); err != nil || !info.Mode().IsRegular() {
			continue
		}
		l = append(l, d)
	}
	slices.Sort(l)
	return l, nil
}

// serviceAddrs returns the addresses of the ip resources of the service, the
// ones a domain must resolve to for its certificate to be asked.
func (t *T) serviceAddrs(ctx context.Context) []net.IP {
	l := make([]net.IP, 0)
	o, ok := t.GetObject().(resourceLister)
	if !ok {
		return l
	}
	for _, r := range o.Resources() {
		if r.ID().DriverGroup() != driver.GroupIP {
			continue
		}
		i, ok := r.(resource.StatusInfoer)
		if !ok {
			continue
		}
		var ip net.IP
		switch v := i.StatusInfo(ctx)["ipaddr"].(type) {
		case net.IP:
			ip = v
		case string:
			ip = net.ParseIP(v)
		}
		if ip != nil {
			l = append(l, ip)
		}
	}
	return l
}

// ready says whether the domain resolves to an address of the service, and
// why not: the ACME directory proves a domain by fetching a token from it,
// and asking a certificate for a domain pointing elsewhere fails, which the
// directory counts against the account.
func ready(ctx context.Context, domain string, addrs []net.IP) (bool, string) {
	if len(addrs) == 0 {
		return false, "the service has no address to check its dns records against"
	}
	resolved, err := net.DefaultResolver.LookupIPAddr(ctx, domain)
	if err != nil {
		return false, fmt.Sprintf("it does not resolve: %s", err)
	}
	have := make([]string, 0, len(resolved))
	for _, a := range resolved {
		for _, want := range addrs {
			if a.IP.Equal(want) {
				return true, ""
			}
		}
		have = append(have, a.IP.String())
	}
	want := make([]string, 0, len(addrs))
	for _, a := range addrs {
		want = append(want, a.String())
	}
	return false, fmt.Sprintf("it resolves to %s, none of the service addresses %s", strings.Join(have, " "), strings.Join(want, " "))
}

// ownedSecs returns the secs domains_to matches the task created, by domain.
func (t *T) ownedSecs(pattern naming.PathPattern) map[string]naming.Path {
	m := make(map[string]naming.Path)
	for _, p := range pattern.InstalledPaths() {
		store, err := object.NewDataStore(p, object.WithVolatile(true))
		if err != nil {
			continue
		}
		owner, err := store.DecodeKey(keyAcmeOwner)
		if err != nil || string(owner) != t.owner() {
			continue
		}
		if d := store.Config().GetString(key.Parse("cn")); d != "" {
			m[d] = p
		}
	}
	return m
}

// settings are the keywords of a sec of a domain the task sets.
func (t *T) settings(domain string) map[string]string {
	return map[string]string{
		"cn":             domain,
		"acme.directory": t.Directory,
		"email":          t.Email,
	}
}

// createSec creates the sec of a domain, marked as the task's.
func (t *T) createSec(pattern naming.PathPattern, domain string) (naming.Path, error) {
	p, err := domainSecPath(pattern, domain)
	if err != nil {
		return p, err
	}
	if p.Exists() {
		return p, fmt.Errorf("%s exists, and is not a sec this task created", p)
	}
	store, err := object.NewDataStore(p)
	if err != nil {
		return p, err
	}
	if err := t.align(store, domain, false); err != nil {
		return p, err
	}
	if err := store.TransactionAddKey(keyAcmeOwner, []byte(t.owner())); err != nil {
		return p, err
	}
	if err := store.Config().Commit(); err != nil {
		return p, err
	}
	t.Log().Infof("%s: created for the domain %s", p, domain)
	return p, nil
}

// align sets the keywords of the sec of a domain to the ones the task asks,
// when they differ, and commits them when asked.
func (t *T) align(store object.DataStore, domain string, commit bool) error {
	ops := make([]string, 0)
	for k, want := range t.settings(domain) {
		have := store.Config().GetString(key.Parse(k))
		if have == want {
			continue
		}
		if want == "" {
			if err := store.Config().Unset(key.Parse(k)); err != nil {
				return err
			}
		} else {
			ops = append(ops, k+"="+want)
		}
	}
	if len(ops) > 0 {
		if err := store.Config().Set(keyop.ParseOps(ops)...); err != nil {
			return err
		}
	}
	if !commit {
		return nil
	}
	return store.Config().Commit()
}

// deleteSec deletes the sec of a domain no file names any longer, on every
// node, as a delete of the object does.
func (t *T) deleteSec(ctx context.Context, p naming.Path, domain string) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	resp, err := c.PostObjectActionDeleteWithResponse(ctx, p.Namespace, p.Kind, p.Name)
	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("delete %s: %s", p, resp.Status())
	}
	t.Log().Infof("%s: delete, no file of domains_from names the domain %s", p, domain)
	return nil
}

// syncDomains creates the secs of the domains the files name, resolving to
// the service, deletes the ones of the domains no file names, and returns the
// secs to renew, and the secs of the task, the ones to renew included.
//
// A domain not resolving to the service yet keeps the sec it has, which a dns
// record going astray for a while does not delete, and gets none: its
// certificate would not be obtained. Nothing is deleted when the files are
// not read.
func (t *T) syncDomains(ctx context.Context) (renew, all []naming.Path, errs error) {
	from, err := parseDomainsFrom(t.DomainsFrom)
	if err != nil {
		return nil, nil, err
	}
	pattern, err := parseDomainsTo(t.DomainsTo, t.Path.Namespace)
	if err != nil {
		return nil, nil, err
	}
	if pattern.Namespace != t.Path.Namespace {
		return nil, nil, fmt.Errorf("domains_to %s: the secs of the domains are in the namespace of the service, %s", t.DomainsTo, t.Path.Namespace)
	}
	owned := t.ownedSecs(pattern)
	for _, p := range owned {
		all = append(all, p)
	}
	domains, err := t.listDomains(ctx, from)
	if err != nil {
		return nil, all, err
	}
	addrs := t.serviceAddrs(ctx)
	created := 0
	for _, d := range domains {
		ok, why := ready(ctx, d, addrs)
		p, has := owned[d]
		switch {
		case has && ok:
			store, err := object.NewDataStore(p)
			if err == nil {
				err = t.align(store, d, true)
			}
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
				continue
			}
			renew = append(renew, p)
		case !ok:
			t.Log().Warnf("%s: pending: %s", d, why)
			errs = errors.Join(errs, fmt.Errorf("%s: pending: %s", d, why))
		case created >= maxCreatedPerRun:
			t.Log().Infof("%s: the sec is created by a next run, this one created %d", d, created)
		default:
			p, err := t.createSec(pattern, d)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("%s: %w", d, err))
				continue
			}
			created++
			renew = append(renew, p)
			all = append(all, p)
		}
	}
	for d, p := range owned {
		if slices.Contains(domains, d) {
			continue
		}
		if err := t.deleteSec(ctx, p, d); err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
		}
	}
	return renew, all, errs
}
