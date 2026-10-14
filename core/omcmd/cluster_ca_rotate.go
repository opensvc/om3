package omcmd

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	// CmdClusterCARotate replaces the key of the cluster ca, system/sec/ca,
	// with no node refusing another meanwhile.
	CmdClusterCARotate struct {
		// Timeout is how long each step waits for every node to have
		// the configurations it wrote.
		Timeout time.Duration

		// DropFormer stops trusting the former ca once every node
		// presents a certificate the new one signed.
		DropFormer bool
	}
)

var (
	caNextPath = naming.Path{Namespace: naming.NsSys, Kind: naming.KindSec, Name: "ca-next"}

	// caKeys are the keys of the sec of a ca the rotation moves to
	// system/sec/ca, and caKeywords its keywords.
	caKeys     = []string{"private_key", "certificate", "certificate_chain", "fullpem", "serial_number"}
	caKeywords = []string{"cn", "c", "st", "l", "o", "ou", "email", "bits", "validity"}

	keyClusterCA = key.New("cluster", "ca")
)

// caRotateValidity is the validity of the new ca, as the one the daemon
// bootstraps.
const caRotateValidity = "10y"

// Run rotates the key of the cluster ca:
//
//  1. a new ca is created as system/sec/ca-next, and the certificate of the
//     current one is kept as system/sec/ca-former-<date>, both trusted by the
//     cluster ca keyword, until every node has them;
//  2. system/sec/ca takes the key and the certificate of the new ca, and
//     system/sec/cert is issued again by it, until every node has them;
//  3. system/sec/ca-next, now system/sec/ca, is no longer listed, and the
//     former ca is not either with --drop-former.
//
// A node verifies its peers, and the api its clients, with the cluster
// authorities: every node trusts the new ca before a certificate it signed is
// presented, and the former one while a certificate it signed is.
func (t *CmdClusterCARotate) Run() error {
	ctx := context.Background()
	c, err := client.New()
	if err != nil {
		return err
	}
	ca, err := object.NewSec(naming.SecCa, object.WithVolatile(false))
	if err != nil {
		return err
	}
	if !ca.HasKey("private_key") {
		return fmt.Errorf("%s has no private_key: a ca installed from elsewhere is rotated from where it was issued", naming.SecCa)
	}
	currentPEM, err := ca.DecodeKey("certificate")
	if err != nil {
		return fmt.Errorf("%s: %w", naming.SecCa, err)
	}
	currentCert, err := parseCertificatePEM(currentPEM)
	if err != nil {
		return fmt.Errorf("%s: %w", naming.SecCa, err)
	}

	// A rotation interrupted once system/sec/ca took the new ca resumes at
	// the step after, its former ca being the one it kept.
	swapped := false
	formerPath, formerCert := naming.Path{}, currentCert
	if caNextPath.Exists() {
		if next, err := object.NewSec(caNextPath, object.WithVolatile(true)); err == nil {
			if b, err := next.DecodeKey("certificate"); err == nil {
				if c, err := parseCertificatePEM(b); err == nil && c.Equal(currentCert) {
					if formerPath, formerCert, err = listedFormer(); err != nil {
						return err
					}
					swapped = true
					fmt.Printf("%s: holds the key of %s already, as a rotation interrupted left it\n", naming.SecCa, caNextPath)
				}
			}
		}
	}
	if t.DropFormer {
		// Checked first: the users of a certificate the former ca signed
		// would be refused once it is not trusted anymore.
		if l := usrsSignedBy(formerCert); len(l) > 0 {
			return fmt.Errorf("the former ca signed the certificate of %s, which the api refuses once it is dropped: renew them with 'om <path> certificate create' after the rotation, or rotate without --drop-former", strings.Join(l, ", "))
		}
	}

	if !swapped {
		// 1. Trust the new ca and the former one.
		next, err := t.newCA(caNextPath)
		if err != nil {
			return err
		}
		if formerPath, err = t.keepFormer(ca, formerCert); err != nil {
			return err
		}
		if err := t.trust(formerPath.String(), caNextPath.String()); err != nil {
			return err
		}
		fmt.Printf("%s: trusts %s and %s\n", keyClusterCA, formerPath, caNextPath)
		if err := t.waitPropagated(ctx, c, naming.Cluster, formerPath, caNextPath); err != nil {
			return err
		}

		// 2. Sign with the new ca.
		if err := t.swap(ca, next); err != nil {
			return err
		}
		fmt.Printf("%s: holds the key of the new ca\n", naming.SecCa)
	}
	paths := []naming.Path{naming.SecCa}
	if err := t.reissueCert(ctx); err != nil {
		fmt.Printf("%s: %s\n", naming.SecCert, err)
	} else {
		paths = append(paths, naming.SecCert)
	}
	if err := t.waitPropagated(ctx, c, paths...); err != nil {
		return err
	}

	// 3. Stop listing the ca the new one was created as, and the former one
	// with --drop-former.
	untrust := []string{caNextPath.String()}
	if t.DropFormer {
		untrust = append(untrust, formerPath.String())
	}
	if err := t.untrust(untrust...); err != nil {
		return err
	}
	if err := t.waitPropagated(ctx, c, naming.Cluster); err != nil {
		return err
	}
	if err := t.deleteSec(caNextPath); err != nil {
		return err
	}
	if t.DropFormer {
		if err := t.deleteSec(formerPath); err != nil {
			return err
		}
		fmt.Printf("%s: the former ca is not trusted anymore\n", keyClusterCA)
	} else {
		fmt.Printf("%s: trusts the former ca as %s still, for the certificates it signed: once they are renewed, stop trusting it with:\n", keyClusterCA, formerPath)
		fmt.Printf("  om cluster config update --set %s-=%s\n", keyClusterCA, formerPath)
		fmt.Printf("  om %s delete\n", formerPath)
	}
	return nil
}

// newCA creates the sec of a new ca at p, or takes the one a rotation
// interrupted created.
func (t *CmdClusterCARotate) newCA(p naming.Path) (object.Sec, error) {
	exists := p.Exists()
	sec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return nil, err
	}
	if exists && sec.HasKey("private_key") && sec.HasKey("certificate") {
		fmt.Printf("%s: exists, as created by a rotation interrupted, taken as the new ca\n", p)
		return sec, nil
	}
	clusterName := ""
	if ccfg, err := object.NewCluster(object.WithVolatile(true)); err == nil {
		clusterName = ccfg.Name()
	}
	ops := []keyop.T{
		*keyop.New(key.New("DEFAULT", "validity"), keyop.Set, caRotateValidity, 0),
		// A cn tells the ca apart from the certificates it signs, which
		// a verifier takes for self-signed when their subjects are the
		// same, and from the former one.
		*keyop.New(key.New("DEFAULT", "cn"), keyop.Set, fmt.Sprintf("%s ca %s", clusterName, time.Now().Format("20060102")), 0),
	}
	if err := sec.Config().PrepareSet(ops...); err != nil {
		return nil, err
	}
	if err := sec.GenCert(); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	fmt.Printf("%s: created, with a new key\n", p)
	return sec, nil
}

// keepFormer keeps the certificate of the former ca, without its key, as a
// sec system/sec/ca-former-<date>, the cluster ca keyword trusting it while
// the certificates it signed are presented.
func (t *CmdClusterCARotate) keepFormer(ca object.Sec, cert *x509.Certificate) (naming.Path, error) {
	base := "ca-former-" + time.Now().Format("20060102")
	var p naming.Path
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		p = naming.Path{Namespace: naming.NsSys, Kind: naming.KindSec, Name: name}
		if !p.Exists() {
			break
		}
		// A rotation interrupted kept it already.
		if sec, err := object.NewSec(p, object.WithVolatile(true)); err == nil {
			if b, err := sec.DecodeKey("certificate"); err == nil {
				if c, err := parseCertificatePEM(b); err == nil && c.Equal(cert) {
					return p, nil
				}
			}
		}
	}
	sec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return p, err
	}
	for _, name := range []string{"certificate", "certificate_chain"} {
		b, err := ca.DecodeKey(name)
		if err != nil {
			return p, fmt.Errorf("%s: %w", naming.SecCa, err)
		}
		if err := sec.TransactionAddKey(name, b); err != nil {
			return p, err
		}
	}
	if err := sec.Config().Commit(); err != nil {
		return p, err
	}
	fmt.Printf("%s: created, the certificate of the former ca\n", p)
	return p, nil
}

// swap gives system/sec/ca the keys and the keywords of the new ca.
func (t *CmdClusterCARotate) swap(ca, next object.Sec) error {
	for _, name := range caKeys {
		if !next.HasKey(name) {
			if err := ca.TransactionRemoveKey(name); err != nil {
				return err
			}
			continue
		}
		b, err := next.DecodeKey(name)
		if err != nil {
			return fmt.Errorf("%s: %w", caNextPath, err)
		}
		if err := ca.TransactionChangeOrAddKey(name, b); err != nil {
			return err
		}
	}
	for _, kw := range caKeywords {
		k := key.New("DEFAULT", kw)
		if s := next.Config().GetString(k); next.Config().HasKey(k) {
			if err := ca.Config().PrepareSet(*keyop.New(k, keyop.Set, s, 0)); err != nil {
				return err
			}
		} else if err := ca.Config().PrepareUnset(k); err != nil {
			return err
		}
	}
	return ca.Config().Commit()
}

// reissueCert issues the certificate of system/sec/cert again, by the ca
// system/sec/ca holds now: its chain holds the certificate of the former
// one. The daemon speaker would too, this one is waited for.
func (t *CmdClusterCARotate) reissueCert(ctx context.Context) error {
	sec, err := object.NewSec(naming.SecCert, object.WithVolatile(false))
	if err != nil {
		return err
	}
	if sec.Config().GetString(key.New("DEFAULT", "ca")) != naming.SecCa.String() {
		return fmt.Errorf("not signed by %s, kept: issue it again from its ca", naming.SecCa)
	}
	result, err := sec.RenewCertificate(ctx, object.CertificateRenewOptions{})
	switch {
	case errors.Is(err, object.ErrCertificateNotIssued):
		return fmt.Errorf("installed from elsewhere, kept: issue it again from its ca")
	case err != nil:
		return err
	case result.Renewed:
		fmt.Printf("%s: issued by the new ca\n", naming.SecCert)
	}
	return nil
}

// trust appends the secs to the cluster ca keyword, the ones it lists
// already kept.
func (t *CmdClusterCARotate) trust(paths ...string) error {
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	current := ccfg.Config().GetStrings(keyClusterCA)
	ops := make([]keyop.T, 0)
	for _, p := range paths {
		if !slices.Contains(current, p) {
			ops = append(ops, *keyop.New(keyClusterCA, keyop.Append, p, 0))
		}
	}
	if len(ops) == 0 {
		return nil
	}
	return ccfg.Config().Set(ops...)
}

// untrust removes the secs from the cluster ca keyword.
func (t *CmdClusterCARotate) untrust(paths ...string) error {
	ccfg, err := object.NewCluster(object.WithVolatile(false))
	if err != nil {
		return err
	}
	current := ccfg.Config().GetStrings(keyClusterCA)
	ops := make([]keyop.T, 0)
	for _, p := range paths {
		if slices.Contains(current, p) {
			ops = append(ops, *keyop.New(keyClusterCA, keyop.Remove, p, 0))
		}
	}
	if len(ops) == 0 {
		return nil
	}
	return ccfg.Config().Set(ops...)
}

// waitPropagated waits until every cluster node has the configuration of
// each path as it is on this node, which the daemon of each node installs
// the files of the cluster authorities from as soon as it has it.
//
// Every node is waited for, a node down among them: a node that would come
// back trusting the former ca alone, once no peer presents a certificate it
// signed, could reach none of them.
func (t *CmdClusterCARotate) waitPropagated(ctx context.Context, c *client.T, paths ...naming.Path) error {
	want := make(map[naming.Path]time.Time)
	for _, p := range paths {
		want[p] = file.ModTime(p.ConfigFile())
	}
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	var laggards []string
	for {
		var err error
		laggards, err = configLaggards(c, want)
		if err == nil && len(laggards) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			if err != nil {
				return fmt.Errorf("wait for the nodes to have %s: %w", naming.Paths(paths), err)
			}
			return fmt.Errorf("wait for the nodes to have %s: timeout, the rotation is interrupted, run it again once they have: %s", naming.Paths(paths), strings.Join(laggards, ", "))
		case <-time.After(time.Second):
		}
	}
}

// configLaggards returns the cluster nodes not having the configuration of a
// path updated at the time want says, as "<path> on <node>".
func configLaggards(c *client.T, want map[naming.Path]time.Time) ([]string, error) {
	b, err := c.NewGetClusterStatus().Get()
	if err != nil {
		return nil, err
	}
	var data clusterdump.Data
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	laggards := make([]string, 0)
	for _, nodename := range data.Cluster.Config.Nodes {
		nodeData, ok := data.Cluster.Node[nodename]
		for p, at := range want {
			inst, found := nodeData.Instance[p.String()]
			if !ok || !found || inst.Config == nil || inst.Config.UpdatedAt.Before(at) {
				laggards = append(laggards, fmt.Sprintf("%s on %s", p, nodename))
			}
		}
	}
	slices.Sort(laggards)
	return laggards, nil
}

// deleteSec deletes the sec at p on every node, as om <path> delete does,
// waiting for it to be deleted.
func (t *CmdClusterCARotate) deleteSec(p naming.Path) error {
	if !p.Exists() {
		return nil
	}
	var cmd CmdObjectDelete
	cmd.ObjectSelector = p.String()
	cmd.Wait = true
	cmd.Time = t.Timeout
	return cmd.Run(naming.KindSec.String())
}

// listedFormer returns the last former ca the cluster ca keyword lists, the
// one a rotation interrupted kept.
func listedFormer() (naming.Path, *x509.Certificate, error) {
	ccfg, err := object.NewCluster(object.WithVolatile(true))
	if err != nil {
		return naming.Path{}, nil, err
	}
	l := ccfg.Config().GetStrings(keyClusterCA)
	for i := len(l) - 1; i >= 0; i-- {
		p, err := naming.ParsePath(l[i])
		if err != nil || p.Namespace != naming.NsSys || p.Kind != naming.KindSec || !strings.HasPrefix(p.Name, "ca-former-") {
			continue
		}
		sec, err := object.NewSec(p, object.WithVolatile(true))
		if err != nil {
			return p, nil, err
		}
		b, err := sec.DecodeKey("certificate")
		if err != nil {
			return p, nil, fmt.Errorf("%s: %w", p, err)
		}
		cert, err := parseCertificatePEM(b)
		if err != nil {
			return p, nil, fmt.Errorf("%s: %w", p, err)
		}
		return p, cert, nil
	}
	return naming.Path{}, nil, fmt.Errorf("%s lists no system/sec/ca-former-<date>, the former ca of the rotation interrupted: add it back, or delete %s to rotate anew", keyClusterCA, caNextPath)
}

// usrsSignedBy returns the users whose certificate ca signed.
func usrsSignedBy(ca *x509.Certificate) []string {
	pattern := naming.PathPattern{Namespace: naming.NsSys, Kind: naming.KindUsr, Name: "*"}
	l := make([]string, 0)
	for _, p := range pattern.InstalledPaths() {
		usr, err := object.NewUsr(p, object.WithVolatile(true))
		if err != nil {
			continue
		}
		b, err := usr.DecodeKey("certificate")
		if err != nil {
			continue
		}
		cert, err := parseCertificatePEM(b)
		if err != nil {
			continue
		}
		if cert.CheckSignatureFrom(ca) == nil {
			l = append(l, p.String())
		}
	}
	return l
}

func parseCertificatePEM(b []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("no certificate")
	}
	return x509.ParseCertificate(block.Bytes)
}
