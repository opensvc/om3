package listener

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/filesystems"
	"github.com/opensvc/om3/v3/util/findmnt"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
)

var (
	certUsr                  = daemonenv.Username
	certGrp                  = daemonenv.Groupname
	certFileMode fs.FileMode = 0600
	certDirMode  fs.FileMode = 0700

	caPath   = naming.SecCa
	certPath = naming.SecCert
)

func (t *T) startCertFS(ctx context.Context) error {
	clusterName, err := getClusterName()
	if err != nil {
		return err
	}
	if err := t.mountCertFS(ctx); err != nil {
		return err
	}

	if err := t.installCaFiles(clusterName); err != nil {
		return err
	}

	if err := t.installCertFiles(clusterName); err != nil {
		return err
	}

	return nil
}

func (t *T) stopCertFS(ctx context.Context) error {
	tmpfs := filesystems.FromType("tmpfs")
	t.log.Infof("unmounting cert fs %s", rawconfig.Paths.Certs)
	if err := tmpfs.Umount(ctx, rawconfig.Paths.Certs); err != nil {
		return err
	}
	t.log.Infof("unmounted cert fs %s", rawconfig.Paths.Certs)
	return nil
}

func (t *T) mountCertFS(ctx context.Context) error {
	if v, err := findmnt.Has(ctx, "none", rawconfig.Paths.Certs); err != nil {
		if err1, ok := err.(*exec.Error); ok {
			if err1.Name == "findmnt" && err1.Err == exec.ErrNotFound {
				// fallback when findmnt is not present
				if exists, err := file.ExistsAndDir(rawconfig.Paths.Certs); err != nil {
					return fmt.Errorf("mount cert fs can't detect file type %s: %w", rawconfig.Paths.Certs, err)
				} else if !exists {
					return fmt.Errorf("mount cert fs can't detect mandatory dir %s", rawconfig.Paths.Certs)
				}
				return nil
			}
			return nil
		}
		return err
	} else if v {
		return nil
	}
	tmpfs := filesystems.FromType("tmpfs")
	t.log.Infof("mounting cert fs %s", rawconfig.Paths.Certs)
	if err := tmpfs.Mount(ctx, "none", rawconfig.Paths.Certs, "rw,nosuid,nodev,noexec,relatime,size=1m"); err != nil {
		return fmt.Errorf("mount cert fs can't mount %s: %w", rawconfig.Paths.Certs, err)
	}
	t.log.Infof("mounted cert fs %s", rawconfig.Paths.Certs)
	return nil
}

func (t *T) installCaFiles(clusterName string) error {
	if !caPath.Exists() {
		if ok, err := t.migrateCaPathV2(clusterName); err != nil {
			return err
		} else if !ok {
			t.log.Infof("install ca files bootstrap initial %s", caPath)
			if err := t.bootStrapCaPath(); err != nil {
				return fmt.Errorf("install ca files can't bootstrap initial %s: %w", caPath, err)
			}
		}
	}
	caSec, err := object.NewSec(caPath, object.WithVolatile(true))
	if err != nil {
		return fmt.Errorf("install ca files can't get %s: %w", caPath, err)
	}

	opt := object.KVInstall{
		AccessControl: object.KVInstallAccessControl{
			Perm:    &certFileMode,
			DirPerm: &certDirMode,
			User:    certUsr,
			Group:   certGrp,
		},
	}

	// ca_certificates for jwt

	opt.FromPattern = "private_key"
	opt.ToPath = daemonenv.CAKeyFile()
	opt.Required = true
	if err := caSec.InstallKeyTo(opt); err != nil {
		return fmt.Errorf("install ca files can't dump ca private_key to %s: %w", opt.ToPath, err)
	} else {
		t.log.Infof("install ca files dump ca private_key to %s", opt.ToPath)
	}

	opt.FromPattern = "certificate_chain"
	opt.ToPath = daemonenv.CACertChainFile()
	opt.Required = true
	if err := caSec.InstallKeyTo(opt); err != nil {
		return fmt.Errorf("install ca files can't dump ca certificate_chain to %s: %w", opt.ToPath, err)
	} else {
		t.log.Infof("install ca files dump ca certificate_chain to %s", opt.ToPath)
	}

	// ca_certificates
	var b []byte
	validCA := make([]string, 0)
	caList := []string{caPath.String()}
	caList = append(caList, cluster.ConfigData.Get().CASecPaths...)
	for _, p := range caList {
		caPath, err := naming.ParsePath(p)
		if err != nil {
			t.log.Warnf("install ca files parse ca %s skipped: %s", p, err)
			continue
		}
		if !caPath.Exists() {
			t.log.Warnf("install ca files skip %s ca: sec object does not exist", caPath)
			continue
		}
		caSec, err := object.NewSec(caPath, object.WithVolatile(true))
		if err != nil {
			return fmt.Errorf("install ca files can't get %s for cert: %w", caPath, err)
		}
		chain, err := caSec.DecodeKey("certificate_chain")
		if err != nil {
			return fmt.Errorf("install ca files can't decode %s certificate_chain for cert: %w", caPath, err)
		}
		b = append(b, chain...)
		validCA = append(validCA, p)
	}
	if len(b) > 0 {
		dst := daemonenv.CAsCertFile()
		if err := os.WriteFile(dst, b, certFileMode); err != nil {
			return fmt.Errorf("install ca files can't create %s: %w", dst, err)
		}
		t.log.Infof("install ca files updated %s ca:%s", dst, validCA)
	}

	// TODO: ca_crl
	return nil
}

func (t *T) installCertFiles(clusterName string) error {
	if !certPath.Exists() {
		if ok, err := t.migrateCertPathV2(clusterName); err != nil {
			return err
		} else if !ok {
			t.log.Infof("install cert files bootstrap initial %s", certPath)
			if err := t.bootStrapCertPath(); err != nil {
				return fmt.Errorf("install cert files can't bootstrap %s: %w", certPath, err)
			}
		}
	}
	certSec, err := object.NewSec(certPath, object.WithVolatile(true))
	if err != nil {
		return fmt.Errorf("install cert files can't get %s: %w", certPath, err)
	}

	opt := object.KVInstall{
		AccessControl: object.KVInstallAccessControl{
			Perm:    &certFileMode,
			DirPerm: &certDirMode,
			User:    certUsr,
			Group:   certGrp,
		},
	}

	opt.FromPattern = "private_key"
	opt.ToPath = daemonenv.KeyFile()
	opt.Required = true
	if err := certSec.InstallKeyTo(opt); err != nil {
		return fmt.Errorf("install cert files can't dump cert private_key to %s: %w", opt.ToPath, err)
	} else {
		t.log.Infof("install cert files dump cert private_key to %s", opt.ToPath)
	}

	opt.FromPattern = "certificate_chain"
	opt.ToPath = daemonenv.CertChainFile()
	opt.Required = true
	if err := certSec.InstallKeyTo(opt); err != nil {
		return fmt.Errorf("install cert files can't dump cert certificate_chain to %s: %w", opt.ToPath, err)
	} else {
		t.log.Infof("install cert files dump cert certificate_chain to %s", opt.ToPath)
	}

	return nil
}

func (t *T) bootStrapCaPath() error {
	p := caPath
	t.log.Infof("bootstrapping ca %s", p)
	caSec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return err
	}
	t.log.Infof("bootstrap ca generating cert %s", p)
	return caSec.GenCert()
}

// migrateCaPathV2 migrates v2 ca to v3+ cert
//
//	return true, nil when v2 ca is migrated to v3
//	return false, nil when no v2 ca exists
//	return true, != nil when migration fails
func (t *T) migrateCaPathV2(clusterName string) (ok bool, err error) {
	caPathV2 := naming.Path{Name: "ca-" + clusterName, Namespace: naming.NsSys, Kind: naming.KindSec}
	ok = caPathV2.Exists()
	if !ok {
		return
	}
	t.log.Infof("migrate ca from %s to %s", caPathV2, caPath)
	if err = os.Rename(caPathV2.ConfigFile(), caPath.ConfigFile()); err != nil {
		err = fmt.Errorf("migrate ca %s to %s: %w", caPathV2, caPath, err)
	}
	return
}

func (t *T) bootStrapCertPath() error {
	p := certPath
	t.log.Infof("create %s", p)
	certSec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return err
	}
	ops := []*keyop.T{
		keyop.New(key.New("DEFAULT", "ca"), keyop.Set, caPath.String(), 0),
		keyop.New(key.New("DEFAULT", "alt_names"), keyop.Set, hostname.Hostname(), 0),
	}
	for _, op := range ops {
		if err := certSec.Config().PrepareSet(*op); err != nil {
			return err
		}
	}
	t.log.Infof("gencert %s", p)
	return certSec.GenCert()
}

func getClusterName() (string, error) {
	clusterCfg, err := object.NewCluster(object.WithVolatile(true))
	if err != nil {
		return "", err
	}
	return clusterCfg.Name(), nil
}

// migrateCertPathV2 migrates v2 cert to v3+ cert
//
//	return true, nil when v2 cert is migrated to v3
//	return false, nil when no v2 cert exists
//	return true, != nil when migration fails
func (t *T) migrateCertPathV2(clusterName string) (hasV2cert bool, err error) {
	certPathV2 := naming.Path{Name: "cert-" + clusterName, Namespace: naming.NsSys, Kind: naming.KindSec}
	hasV2cert = certPathV2.Exists()
	if !hasV2cert {
		return
	}
	t.log.Infof("migrate cert %s to %s", certPathV2, certPath)
	if err = os.Rename(certPathV2.ConfigFile(), certPath.ConfigFile()); err != nil {
		t.log.Errorf("migrate cert %s to %s: %s", certPathV2, certPath, err)
		return
	}
	certSec, err2 := object.NewSec(certPath, object.WithVolatile(false))
	if err2 != nil {
		err = err2
		t.log.Errorf("create %s: %s", certPath, err)
		return
	}
	t.log.Infof("update migrated cert ca keyword to %s", caPath)
	op := keyop.New(key.New("DEFAULT", "ca"), keyop.Set, caPath.String(), 0)
	err = certSec.Config().Set(*op)
	return
}
