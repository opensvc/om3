package daemonauth

import (
	crypto_x509 "crypto/x509"
	"encoding/pem"
	"io/ioutil"

	"github.com/rs/zerolog/log"
	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/strategies/x509"

	"opensvc.com/opensvc/daemon/daemonenv"
)

func initX509() auth.Strategy {
	log.Logger.Info().Msg("init x509 auth strategy")
	opts := CreateVerifyOptions()
	strategy := x509.New(opts)
	return strategy
}

func ParseCertificate() *crypto_x509.Certificate {
	ca, err := ioutil.ReadFile(daemonenv.CACertFile())
	if err != nil {
		log.Logger.Error().Err(err).Msg("read ca certificate")
		return nil
	}
	p, _ := pem.Decode(ca)
	if p == nil {
		log.Logger.Error().Msg("failed to decode PEM ca certificate")
		return nil
	}
	cert, err := crypto_x509.ParseCertificate(p.Bytes)
	if err != nil {
		log.Logger.Error().Err(err).Msg("parse ca certificate")
		return nil
	}
	return cert
}

func CreateVerifyOptions() crypto_x509.VerifyOptions {
	opts := crypto_x509.VerifyOptions{}
	opts.KeyUsages = []crypto_x509.ExtKeyUsage{crypto_x509.ExtKeyUsageClientAuth}
	opts.Roots = crypto_x509.NewCertPool()
	opts.Roots.AddCert(ParseCertificate())
	return opts
}
