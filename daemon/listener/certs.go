package listener

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/ccfg"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/filesystems"
	"github.com/opensvc/om3/util/findmnt"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/key"
)

var (
	certUsr                  = daemonenv.Username
	certGrp                  = daemonenv.Groupname
	certFileMode fs.FileMode = 0600
	certDirMode  fs.FileMode = 0700

	caPath   = naming.Path{Name: "ca", Namespace: "system", Kind: naming.KindSec}
	certPath = naming.Path{Name: "cert", Namespace: "system", Kind: naming.KindSec}
)

func startCertFS() error {
	clusterName, err := getClusterName()
	if err != nil {
		return err
	}
	if err := mountCertFS(); err != nil {
		return err
	}

	if err := installCaFiles(clusterName); err != nil {
		return err
	}

	if err := installCertFiles(clusterName); err != nil {
		return err
	}

	return nil
}

func stopCertFS() error {
	tmpfs := filesystems.FromType("tmpfs")
	return tmpfs.Umount(rawconfig.Paths.Certs)
}

func mountCertFS() error {
	if v, err := findmnt.Has("none", rawconfig.Paths.Certs); err != nil {
		if err1, ok := err.(*exec.Error); ok {
			if err1.Name == "findmnt" && err1.Err == exec.ErrNotFound {
				// fallback when findmnt is not present
				if exists, err := file.ExistsAndDir(rawconfig.Paths.Certs); err != nil {
					return err
				} else if !exists {
					return fmt.Errorf("missing mandatory dir %s", rawconfig.Paths.Certs)
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
	if err := tmpfs.Mount("none", rawconfig.Paths.Certs, "rw,nosuid,nodev,noexec,relatime,size=1m"); err != nil {
		return err
	}
	return nil
}

func installCaFiles(clusterName string) error {
	if !caPath.Exists() {
		if ok, err := migrateCaPathV2(clusterName); err != nil {
			return err
		} else if !ok {
			log.Logger.Info().Msgf("bootstrap initial %s", caPath)
			if err := bootStrapCaPath(caPath); err != nil {
				log.Logger.Error().Err(err).Msgf("bootStrapCaPath %s", caPath)
				return err
			}
		}
	}
	caSec, err := object.NewSec(caPath, object.WithVolatile(true))
	if err != nil {
		log.Logger.Error().Err(err).Msgf("create %s", caPath)
		return err
	}

	// ca_certificates for jwt
	dst := daemonenv.CAKeyFile()

	if err := caSec.InstallKeyTo("private_key", dst, &certFileMode, &certDirMode, certUsr, certGrp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	dst = daemonenv.CACertChainFile()
	if err := caSec.InstallKeyTo("certificate_chain", dst, &certFileMode, &certDirMode, certUsr, certGrp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	// ca_certificates
	var b []byte
	validCA := make([]string, 0)
	caList := []string{caPath.String()}
	caList = append(caList, ccfg.Get().CASecPaths...)
	for _, p := range caList {
		caPath, err := naming.Parse(p)
		if err != nil {
			log.Logger.Warn().Err(err).Msgf("parse ca %s", p)
			continue
		}
		if !caPath.Exists() {
			log.Logger.Warn().Msgf("skip %s ca: sec object does not exist", caPath)
			continue
		}
		caSec, err := object.NewSec(caPath, object.WithVolatile(true))
		if err != nil {
			return err
		}
		chain, err := caSec.DecodeKey("certificate_chain")
		if err != nil {
			return err
		}
		b = append(b, chain...)
		validCA = append(validCA, p)
	}
	if len(b) > 0 {
		dst := daemonenv.CAsCertFile()
		if err := os.WriteFile(dst, b, certFileMode); err != nil {
			return err
		}
		log.Logger.Info().Strs("ca", validCA).Msgf("installed %s", dst)
	}

	// TODO: ca_crl
	return nil
}

func installCertFiles(clusterName string) error {
	if !certPath.Exists() {
		if ok, err := migrateCertPathV2(clusterName); err != nil {
			return err
		} else if !ok {
			log.Logger.Info().Msgf("bootstrap initial %s", certPath)
			if err := bootStrapCertPath(certPath, caPath); err != nil {
				return err
			}
		}
	}
	certSec, err := object.NewSec(certPath, object.WithVolatile(true))
	if err != nil {
		log.Logger.Error().Err(err).Msgf("create %s", certPath)
		return err
	}

	dst := daemonenv.KeyFile()
	if err := certSec.InstallKeyTo("private_key", dst, &certFileMode, &certDirMode, certUsr, certGrp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}
	dst = daemonenv.CertChainFile()
	if err := certSec.InstallKeyTo("certificate_chain", dst, &certFileMode, &certDirMode, certUsr, certGrp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	dst = daemonenv.CertFile()
	if err := certSec.InstallKeyTo("certificate", dst, &certFileMode, &certDirMode, certUsr, certGrp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}
	return nil
}

func bootStrapCaPath(p naming.Path) error {
	log.Logger.Info().Msgf("create %s", p)
	caSec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return err
	}
	log.Logger.Info().Msgf("gencert %s", p)
	return caSec.GenCert()
}

// migrateCaPathV2 migrates v2 ca to v3+ cert
//
//	return true, nil when v2 ca is migrated to v3
//	return false, nil when no v2 ca exists
//	return true, != nil when migration fails
func migrateCaPathV2(clusterName string) (ok bool, err error) {
	caPathV2 := naming.Path{Name: "ca-" + clusterName, Namespace: "system", Kind: naming.KindSec}
	ok = caPathV2.Exists()
	if !ok {
		return
	}
	log.Logger.Info().Msgf("migrate ca from %s to %s", caPathV2, caPath)
	if err = os.Rename(caPathV2.ConfigFile(), caPath.ConfigFile()); err != nil {
		log.Logger.Error().Err(err).Msgf("migrate ca %s to %s", caPathV2, caPath)
	}
	return
}

func bootStrapCertPath(p naming.Path, caPath naming.Path) error {
	log.Logger.Info().Msgf("create %s", p)
	certSec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return err
	}
	ops := []*keyop.T{
		keyop.New(key.New("DEFAULT", "ca"), keyop.Set, caPath.String(), 0),
		keyop.New(key.New("DEFAULT", "alt_names"), keyop.Set, hostname.Hostname(), 0),
	}
	for _, op := range ops {
		if err := certSec.Config().Set(*op); err != nil {
			return err
		}
	}
	log.Logger.Info().Msgf("gencert %s", p)
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
func migrateCertPathV2(clusterName string) (hasV2cert bool, err error) {
	certPathV2 := naming.Path{Name: "cert-" + clusterName, Namespace: "system", Kind: naming.KindSec}
	hasV2cert = certPathV2.Exists()
	if !hasV2cert {
		return
	}
	log.Logger.Info().Msgf("migrate cert %s to %s", certPathV2, certPath)
	if err = os.Rename(certPathV2.ConfigFile(), certPath.ConfigFile()); err != nil {
		log.Logger.Error().Err(err).Msgf("migrate cert %s to %s", certPathV2, certPath)
		return
	}
	certSec, err2 := object.NewSec(certPath, object.WithVolatile(false))
	if err2 != nil {
		err = err2
		log.Logger.Error().Err(err).Msgf("create %s", certPath)
		return
	}
	log.Logger.Info().Msgf("update migrated cert ca keyword to %s", caPath)
	op := keyop.New(key.New("DEFAULT", "ca"), keyop.Set, caPath.String(), 0)
	if err = certSec.Config().Set(*op); err != nil {
		return
	}
	err = certSec.Config().Commit()
	return
}
