package listener

import (
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/keyop"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/filesystems"
	"opensvc.com/opensvc/util/findmnt"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
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
				if !file.ExistsAndDir(rawconfig.Paths.Certs) {
					return errors.New("missing mandatory dir " + rawconfig.Paths.Certs)
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
	var (
		caPath path.T
	)
	caPath, err := getSecCaPath(clusterName)
	if err != nil {
		return err
	}
	if !caPath.Exists() {
		log.Logger.Info().Msgf("bootstrap initial %s", caPath)
		if err := bootStrapCaPath(caPath); err != nil {
			log.Logger.Error().Err(err).Msgf("bootStrapCaPath %s", caPath)
			return err
		}
	}
	caSec, err := object.NewSec(caPath, object.WithVolatile(true))
	if err != nil {
		log.Logger.Error().Err(err).Msgf("create %s", caPath)
		return err
	}

	err, usr, grp, fmode, dmode := getCertFilesModes()
	if err != nil {
		return err
	}

	// ca_certificates for jwt
	dst := daemonenv.CAKeyFile()

	if err := caSec.InstallKeyTo("private_key", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	dst = daemonenv.CACertChainFile()
	if err := caSec.InstallKeyTo("certificate_chain", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	// ca_certificates
	var b []byte
	validCA := make([]string, 0)
	caList := []string{caPath.String()}
	caList = append(caList, strings.Fields(rawconfig.ClusterSection().CASecPaths)...)
	for _, p := range caList {
		caPath, err := path.Parse(p)
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
		if err := os.WriteFile(dst, b, fmode); err != nil {
			return err
		}
		log.Logger.Info().Strs("ca", validCA).Msgf("installed %s", dst)
	}

	// TODO: ca_crl
	return nil
}

func installCertFiles(clusterName string) error {
	certPath, err := getSecCertPath(clusterName)
	if err != nil {
		return err
	}
	caPath, err := getSecCaPath(clusterName)
	if err != nil {
		return err
	}
	if !certPath.Exists() {
		log.Logger.Info().Msgf("bootstrap initial %s", certPath)
		if err := bootStrapCertPath(certPath, caPath); err != nil {
			return err
		}
	}
	certSec, err := object.NewSec(certPath, object.WithVolatile(true))
	if err != nil {
		log.Logger.Error().Err(err).Msgf("create %s", certPath)
		return err
	}
	err, usr, grp, fmode, dmode := getCertFilesModes()
	if err != nil {
		return err
	}
	dst := daemonenv.KeyFile()
	if err := certSec.InstallKeyTo("private_key", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}
	dst = daemonenv.CertChainFile()
	if err := certSec.InstallKeyTo("certificate_chain", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	dst = daemonenv.CertFile()
	if err := certSec.InstallKeyTo("certificate", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}
	return nil
}

func getCertFilesModes() (err error, usr *user.User, grp *user.Group, fmode, dmode fs.FileMode) {
	usr, err = user.Lookup("root")
	if err != nil {
		return
	}
	grp, err = user.LookupGroupId(usr.Gid)
	if err != nil {
		return
	}
	fmode = 0600
	dmode = 0700
	return
}

func bootStrapCaPath(p path.T) error {
	log.Logger.Info().Msgf("create %s", p)
	caSec, err := object.NewSec(p, object.WithVolatile(false))
	if err != nil {
		return err
	}
	log.Logger.Info().Msgf("gencert %s", p)
	return caSec.GenCert()
}

func bootStrapCertPath(p path.T, caPath path.T) error {
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
	clusterCfg, err := object.NewCcfg(path.Cluster, object.WithVolatile(true))
	if err != nil {
		return "", err
	}
	return clusterCfg.Name(), nil
}

func getSecCaPath(clusterName string) (path.T, error) {
	return path.Parse("system/sec/ca-" + clusterName)
}

func getSecCertPath(clusterName string) (path.T, error) {
	return path.Parse("system/sec/cert-" + clusterName)
}
