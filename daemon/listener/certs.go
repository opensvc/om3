package listener

import (
	"io/fs"
	"os"
	"os/user"
	"strings"

	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/daemonenv"
	"opensvc.com/opensvc/util/filesystems"
	"opensvc.com/opensvc/util/findmnt"
)

func mountCertFS() error {
	if v, err := findmnt.Has("none", rawconfig.Paths.Certs); err != nil {
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

func startCertFS() error {
	if err := mountCertFS(); err != nil {
		return err
	}
	certPath, _ := path.Parse("system/sec/cert-" + rawconfig.ClusterSection().Name)
	certSec, err := object.NewSec(certPath, object.WithVolatile(true))
	if err != nil {
		return err
	}

	usr, err := user.Lookup("root")
	if err != nil {
		return err
	}
	grp, err := user.LookupGroupId(usr.Gid)
	if err != nil {
		return err
	}
	var fmode fs.FileMode = 0600
	var dmode fs.FileMode = 0700
	dst := daemonenv.KeyFile()
	if err := certSec.InstallKeyTo("private_key", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}
	dst = daemonenv.CertFile()
	if err := certSec.InstallKeyTo("certificate_chain", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	// ca_certificates
	b := []byte{}
	validCA := make([]string, 0)
	caList := []string{"system/sec/ca-" + rawconfig.ClusterSection().Name}
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

	// ca_certificates for jwt
	dst = daemonenv.CAKeyFile()
	caPath, _ := path.Parse("system/sec/ca-" + rawconfig.ClusterSection().Name)
	caSec, err := object.NewSec(caPath, object.WithVolatile(true))
	if err != nil {
		return err
	}

	if err := caSec.InstallKeyTo("private_key", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	dst = daemonenv.CACertFile()
	if err := caSec.InstallKeyTo("certificate_chain", dst, &fmode, &dmode, usr, grp); err != nil {
		return err
	} else {
		log.Logger.Info().Msgf("installed %s", dst)
	}

	return nil
}

func stopCertFS() error {
	tmpfs := filesystems.FromType("tmpfs")
	return tmpfs.Umount(rawconfig.Paths.Certs)
}
